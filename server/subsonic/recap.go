package subsonic

import (
	"context"
	"net/http"
	"strconv"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/subsonic/responses"
	"github.com/navidrome/navidrome/utils/req"
)

const (
	defaultRecapLimit = 20
	maxRecapLimit     = 100
)

// recapDateRange resolves the [from, to] window for a getRecap request from
// its "from"/"to" params (RFC3339, e.g. 2026-01-01T00:00:00Z, or unix
// seconds; want a full year? pass from=2026-01-01T00:00:00Z&to=2026-12-31T23:59:59Z).
// An omitted "from" defaults to the zero time.Time (unbounded start - always
// <= any real submission_time, so it behaves as "since the beginning"). An
// omitted "to" defaults to now, *not* the zero time.Time - the zero time is
// year 1, and used as an upper bound that would match nothing. With both
// omitted, the range is "everything so far".
//
// Mirrors server/nativeapi/stats.go's parseStatsRange, kept separate since
// the two packages don't share request-parsing helpers.
func recapDateRange(p *req.Values) (from, to time.Time, err error) {
	if fromStr := p.StringOr("from", ""); fromStr != "" {
		if from, err = recapParseTime(fromStr); err != nil {
			return
		}
	}
	if toStr := p.StringOr("to", ""); toStr != "" {
		to, err = recapParseTime(toStr)
	} else {
		to = time.Now().UTC()
	}
	return
}

func recapParseTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	return time.Parse(time.RFC3339, s)
}

func recapLimit(p *req.Values) int {
	limit := p.IntOr("count", defaultRecapLimit)
	if limit <= 0 {
		limit = defaultRecapLimit
	}
	return min(limit, maxRecapLimit)
}

// GetRecap is a fork-only, non-standard Subsonic extension (no such endpoint
// exists in the real Subsonic/OpenSubsonic spec - same idea as
// getSonicSimilarTracks/findSonicPath in sonic_similarity.go) that returns
// listening-stats/recap aggregates for a period in one combined payload:
// overall summary, top songs, top artists, and the VibeNet taste profile.
func (api *Router) GetRecap(r *http.Request) (*responses.Subsonic, error) {
	ctx := r.Context()
	p := req.Params(r)
	from, to, err := recapDateRange(p)
	if err != nil {
		return nil, newError(responses.ErrorGeneric, "invalid from/to: %s", err)
	}
	limit := recapLimit(p)

	repo := api.ds.Scrobble(ctx)

	summary, err := repo.Summary(from, to)
	if err != nil {
		return nil, err
	}
	topSongs, err := repo.TopSongs(from, to, limit)
	if err != nil {
		return nil, err
	}
	topArtists, err := repo.TopArtists(from, to, limit)
	if err != nil {
		return nil, err
	}
	taste, err := repo.TasteProfile(from, to)
	if err != nil {
		return nil, err
	}

	topSongsResp, err := recapTopSongsResponse(ctx, api.ds, topSongs)
	if err != nil {
		return nil, err
	}

	response := newResponse()
	response.Recap = &responses.Recap{
		Summary:      recapSummaryResponse(summary),
		TopSongs:     topSongsResp,
		TopArtists:   recapTopArtistsResponse(topArtists),
		TasteProfile: recapTasteProfileResponse(taste),
	}
	return response, nil
}

// loadMediaFilesByID fetches the MediaFiles for the given IDs, keyed by ID.
// Order is restored by the caller, which already has the ranked ID list from
// TopSongs. IDs with no matching row (e.g. purged since the scrobble was
// recorded) are simply absent from the result - mirrors the existing
// playQueueRepository.loadTracks pattern (persistence/playqueue_repository.go).
func loadMediaFilesByID(ctx context.Context, ds model.DataStore, ids []string) (map[string]model.MediaFile, error) {
	if len(ids) == 0 {
		return nil, nil
	}
	mfs, err := ds.MediaFile(ctx).GetAll(model.QueryOptions{Filters: Eq{"media_file.id": ids}})
	if err != nil {
		return nil, err
	}
	byID := make(map[string]model.MediaFile, len(mfs))
	for _, mf := range mfs {
		byID[mf.ID] = mf
	}
	return byID, nil
}

func recapSummaryResponse(s model.ListenSummary) responses.RecapSummary {
	return responses.RecapSummary{
		PlayCount:     s.PlayCount,
		TotalMinutes:  s.TotalMinutes,
		UniqueSongs:   s.UniqueSongs,
		UniqueArtists: s.UniqueArtists,
	}
}

// recapTopSongsResponse hydrates each TopSong's MediaFileID into a full
// Child (same info getSong.view returns) via childFromMediaFile, preserving
// TopSongs' ranked order. A song whose MediaFile is no longer found (purged
// since the scrobble was recorded) is silently omitted, not errored.
func recapTopSongsResponse(ctx context.Context, ds model.DataStore, songs []model.TopSong) (responses.Array[responses.RecapTopSong], error) {
	ids := make([]string, len(songs))
	for i, s := range songs {
		ids[i] = s.MediaFileID
	}
	byID, err := loadMediaFilesByID(ctx, ds, ids)
	if err != nil {
		return nil, err
	}
	resp := make(responses.Array[responses.RecapTopSong], 0, len(songs))
	for _, s := range songs {
		mf, ok := byID[s.MediaFileID]
		if !ok {
			continue
		}
		resp = append(resp, responses.RecapTopSong{
			Entry:        childFromMediaFile(ctx, mf),
			PlayCount:    s.PlayCount,
			TotalMinutes: s.TotalMinutes,
		})
	}
	return resp, nil
}

func recapTopArtistsResponse(artists []model.TopArtist) responses.Array[responses.RecapTopArtist] {
	resp := make(responses.Array[responses.RecapTopArtist], len(artists))
	for i, a := range artists {
		resp[i] = responses.RecapTopArtist{
			ArtistId:     a.ArtistID,
			Name:         a.Name,
			PlayCount:    a.PlayCount,
			TotalMinutes: a.TotalMinutes,
		}
	}
	return resp
}

func recapTasteProfileResponse(t model.TasteProfile) responses.RecapTasteProfile {
	return responses.RecapTasteProfile{
		Acousticness:     t.Acousticness,
		Danceability:     t.Danceability,
		Energy:           t.Energy,
		Instrumentalness: t.Instrumentalness,
		Liveness:         t.Liveness,
		Speechiness:      t.Speechiness,
		Valence:          t.Valence,
		TrackCount:       t.TrackCount,
	}
}
