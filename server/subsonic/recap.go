package subsonic

import (
	"net/http"
	"time"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/subsonic/responses"
	"github.com/navidrome/navidrome/utils/req"
)

const (
	defaultRecapLimit = 20
	maxRecapLimit     = 100
)

// recapYearRange resolves the [from, to] window for a getRecap request from
// its "year" param (Jan 1 00:00:00 - Dec 31 23:59:59 UTC of that year),
// defaulting to the current year when omitted. Mirrors
// server/nativeapi/stats.go's parseStatsRange, kept separate since the two
// packages don't share request-parsing helpers.
func recapYearRange(p *req.Values) (from, to time.Time) {
	year := p.IntOr("year", time.Now().UTC().Year())
	from = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	to = time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Second)
	return
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
	from, to := recapYearRange(p)
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

	response := newResponse()
	response.Recap = &responses.Recap{
		Summary:      recapSummaryResponse(summary),
		TopSongs:     recapTopSongsResponse(topSongs),
		TopArtists:   recapTopArtistsResponse(topArtists),
		TasteProfile: recapTasteProfileResponse(taste),
	}
	return response, nil
}

func recapSummaryResponse(s model.ListenSummary) responses.RecapSummary {
	return responses.RecapSummary{
		PlayCount:     s.PlayCount,
		TotalMinutes:  s.TotalMinutes,
		UniqueSongs:   s.UniqueSongs,
		UniqueArtists: s.UniqueArtists,
	}
}

func recapTopSongsResponse(songs []model.TopSong) responses.Array[responses.RecapTopSong] {
	resp := make(responses.Array[responses.RecapTopSong], len(songs))
	for i, s := range songs {
		resp[i] = responses.RecapTopSong{
			MediaFileId:  s.MediaFileID,
			Title:        s.Title,
			Artist:       s.Artist,
			PlayCount:    s.PlayCount,
			TotalMinutes: s.TotalMinutes,
		}
	}
	return resp
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
