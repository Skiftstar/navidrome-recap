package persistence

import (
	"context"
	"time"

	. "github.com/Masterminds/squirrel"
	"github.com/deluan/rest"
	"github.com/navidrome/navidrome/model"
	"github.com/pocketbase/dbx"
)

type scrobbleRepository struct {
	sqlRepository
}

func fromTs(_ string, value any) Sqlizer {
	return GtOrEq{"scrobbles.submission_time": value}
}

func toTs(_ string, value any) Sqlizer {
	return LtOrEq{"scrobbles.submission_time": value}
}

func (r *scrobbleRepository) baseQuery(options ...model.QueryOptions) SelectBuilder {
	user := loggedUser(r.ctx)

	return r.newSelect(options...).
		Columns("id", "media_file_id", "submission_time").
		Where(Eq{"scrobbles.user_id": user.ID})
}

func NewScrobbleRepository(ctx context.Context, db dbx.Builder) model.ScrobbleRepository {
	r := &scrobbleRepository{}
	r.ctx = ctx
	r.db = db
	r.tableName = "scrobbles"
	r.registerModel(&model.Scrobble{}, map[string]filterFunc{
		"from": fromTs,
		"to":   toTs,
	})
	r.setSortMappings(map[string]string{
		"submission_time": "submission_time",
	})
	return r
}

func (r *scrobbleRepository) RecordScrobble(mediaFileID string, submissionTime time.Time) error {
	userID := loggedUser(r.ctx).ID
	values := map[string]any{
		"media_file_id":   mediaFileID,
		"user_id":         userID,
		"submission_time": submissionTime.Unix(),
	}
	insert := Insert(r.tableName).SetMap(values)
	_, err := r.executeSQL(insert)
	return err
}

func (r *scrobbleRepository) CountAll(options ...model.QueryOptions) (int64, error) {
	return r.count(r.baseQuery(), options...)
}

func (r *scrobbleRepository) Count(options ...rest.QueryOptions) (int64, error) {
	return r.CountAll(r.parseRestOptions(r.ctx, options...))
}

func (r *scrobbleRepository) Get(id string) (*model.Scrobble, error) {
	sel := r.baseQuery().Where(Eq{"id": id})
	var res model.Scrobble
	err := r.queryOne(sel, &res)
	return &res, err
}

func (r *scrobbleRepository) GetAll(options ...model.QueryOptions) (model.Scrobbles, error) {
	sel := r.baseQuery(options...)
	var scrobbles model.Scrobbles
	err := r.queryAll(sel, &scrobbles)
	return scrobbles, err
}

// scrobbleFilter is the (user, date range) filter shared by every recap/stats
// aggregate query below. Expects "s" as the scrobbles table alias.
func (r *scrobbleRepository) scrobbleFilter(from, to time.Time) Sqlizer {
	return And{
		Eq{"s.user_id": loggedUser(r.ctx).ID},
		GtOrEq{"s.submission_time": from.Unix()},
		LtOrEq{"s.submission_time": to.Unix()},
	}
}

// scrobbleJoinMediaFile is the base query for aggregates that only need
// scrobbles+media_file (no artist attribution): TopSongs, Summary's totals,
// and TasteProfile. Callers add Columns()/GroupBy()/etc.
func (r *scrobbleRepository) scrobbleJoinMediaFile(from, to time.Time) SelectBuilder {
	return Select().
		From("scrobbles s").
		Join("media_file mf on mf.id = s.media_file_id").
		Where(r.scrobbleFilter(from, to))
}

func (r *scrobbleRepository) TopSongs(from, to time.Time, limit int) ([]model.TopSong, error) {
	sel := r.scrobbleJoinMediaFile(from, to).
		Columns(
			"s.media_file_id", "mf.title", "mf.artist",
			"count(*) as play_count",
			"coalesce(sum(mf.duration), 0) / 60.0 as total_minutes",
		).
		GroupBy("s.media_file_id").
		OrderBy("play_count desc").
		Limit(uint64(limit))
	var res []model.TopSong
	err := r.queryAll(sel, &res)
	return res, err
}

// TopArtists deliberately joins media_file_artists (filtered to the "artist"
// role): a track credited to multiple artists contributes one row per
// credited artist, so each gets their own play count. That's correct here,
// but would over-count a track-level total (see Summary, which avoids this
// join for its overall totals).
func (r *scrobbleRepository) TopArtists(from, to time.Time, limit int) ([]model.TopArtist, error) {
	sel := Select(
		"mfa.artist_id", "ar.name",
		"count(*) as play_count",
		"coalesce(sum(mf.duration), 0) / 60.0 as total_minutes",
	).
		From("scrobbles s").
		Join("media_file mf on mf.id = s.media_file_id").
		Join("media_file_artists mfa on mfa.media_file_id = s.media_file_id and mfa.role = ?", model.RoleArtist.String()).
		Join("artist ar on ar.id = mfa.artist_id").
		Where(r.scrobbleFilter(from, to)).
		GroupBy("mfa.artist_id").
		OrderBy("play_count desc").
		Limit(uint64(limit))
	var res []model.TopArtist
	err := r.queryAll(sel, &res)
	return res, err
}

// Summary runs two queries rather than one: the overall totals (play count,
// minutes, unique songs) must NOT join media_file_artists, since a
// multi-artist track would multiply those rows and inflate the totals - only
// the unique-artist count needs that join, so it's kept separate.
func (r *scrobbleRepository) Summary(from, to time.Time) (model.ListenSummary, error) {
	var totals struct {
		PlayCount    int64   `db:"play_count"`
		TotalMinutes float64 `db:"total_minutes"`
		UniqueSongs  int64   `db:"unique_songs"`
	}
	totalsSel := r.scrobbleJoinMediaFile(from, to).
		Columns(
			"count(*) as play_count",
			"coalesce(sum(mf.duration), 0) / 60.0 as total_minutes",
			"count(distinct s.media_file_id) as unique_songs",
		)
	if err := r.queryOne(totalsSel, &totals); err != nil {
		return model.ListenSummary{}, err
	}

	var artists struct {
		UniqueArtists int64 `db:"unique_artists"`
	}
	artistsSel := Select("count(distinct mfa.artist_id) as unique_artists").
		From("scrobbles s").
		Join("media_file_artists mfa on mfa.media_file_id = s.media_file_id and mfa.role = ?", model.RoleArtist.String()).
		Where(r.scrobbleFilter(from, to))
	if err := r.queryOne(artistsSel, &artists); err != nil {
		return model.ListenSummary{}, err
	}

	return model.ListenSummary{
		PlayCount:     totals.PlayCount,
		TotalMinutes:  totals.TotalMinutes,
		UniqueSongs:   totals.UniqueSongs,
		UniqueArtists: artists.UniqueArtists,
	}, nil
}

// TasteProfile averages the VibeNet/Part-A audio-feature tags (see
// resources/mappings.yaml docs / conf.Server.Tags) across every scrobbled
// track in the period. Each json_extract yields NULL for a track that
// doesn't have that tag, and SQL avg() silently skips NULLs - so a field is
// nil only when none of the scrobbled tracks had that tag, not when some did.
//
// The tags column stores each value as a {"id":..., "value":...} object (see
// marshalTags/dbTag in sql_tags.go), not a bare string, so every path below
// ends in ".value" - '$.acousticness[0]' alone would extract the whole
// object and CAST(... AS REAL) it to NULL.
func (r *scrobbleRepository) TasteProfile(from, to time.Time) (model.TasteProfile, error) {
	sel := r.scrobbleJoinMediaFile(from, to).
		Columns(
			"avg(cast(json_extract(mf.tags, '$.acousticness[0].value') as real)) as acousticness",
			"avg(cast(json_extract(mf.tags, '$.danceability[0].value') as real)) as danceability",
			"avg(cast(json_extract(mf.tags, '$.energy[0].value') as real)) as energy",
			"avg(cast(json_extract(mf.tags, '$.instrumentalness[0].value') as real)) as instrumentalness",
			"avg(cast(json_extract(mf.tags, '$.liveness[0].value') as real)) as liveness",
			"avg(cast(json_extract(mf.tags, '$.speechiness[0].value') as real)) as speechiness",
			"avg(cast(json_extract(mf.tags, '$.valence[0].value') as real)) as valence",
			"count(*) as track_count",
		)
	var res model.TasteProfile
	err := r.queryOne(sel, &res)
	return res, err
}

func (r *scrobbleRepository) Read(id string) (any, error) {
	return r.Get(id)
}

func (r *scrobbleRepository) ReadAll(options ...rest.QueryOptions) (any, error) {
	return r.GetAll(r.parseRestOptions(r.ctx, options...))
}

func (r *scrobbleRepository) EntityName() string {
	return "scrobble"
}

func (r *scrobbleRepository) NewInstance() any {
	return &model.Scrobble{}
}

var _ model.ScrobbleRepository = (*scrobbleRepository)(nil)
var _ model.ResourceRepository = (*scrobbleRepository)(nil)
