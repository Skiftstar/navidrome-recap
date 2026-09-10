package model

import "time"

type Scrobble struct {
	ID             int64  `structs:"id" json:"id"`
	MediaFileID    string `structs:"media_file_id" json:"mediaFileId"`
	UserID         string `json:"-"`
	SubmissionTime int64  `structs:"submission_time" json:"submissionTime"`
}

type ScrobbleRepository interface {
	CountAll(options ...QueryOptions) (int64, error)
	Get(id string) (*Scrobble, error)
	GetAll(options ...QueryOptions) (Scrobbles, error)
	RecordScrobble(mediaFileID string, submissionTime time.Time) error

	// TopSongs returns, for the logged-in user, the most-scrobbled songs whose
	// submission time falls within [from, to], ordered by play count descending.
	TopSongs(from, to time.Time, limit int) ([]TopSong, error)
	// TopArtists returns, for the logged-in user, the most-scrobbled artists
	// (attributed via the "artist" role in media_file_artists) whose
	// submission time falls within [from, to], ordered by play count descending.
	TopArtists(from, to time.Time, limit int) ([]TopArtist, error)
	// Summary returns overall listening totals for the logged-in user within [from, to].
	Summary(from, to time.Time) (ListenSummary, error)
	// TasteProfile returns the average VibeNet/audio-feature tag values across
	// every scrobbled track (that has those tags) for the logged-in user within [from, to].
	TasteProfile(from, to time.Time) (TasteProfile, error)
}

type Scrobbles []Scrobble

// TotalMinutes for TopSong/TopArtist/ListenSummary is an approximation: each
// scrobble is treated as one full listen of the track's duration (there is no
// per-scrobble "seconds actually played" recorded), then summed and converted
// from seconds to minutes.

type TopSong struct {
	MediaFileID  string  `db:"media_file_id"  json:"mediaFileId"`
	Title        string  `db:"title"          json:"title"`
	Artist       string  `db:"artist"         json:"artist"`
	PlayCount    int64   `db:"play_count"     json:"playCount"`
	TotalMinutes float64 `db:"total_minutes"  json:"totalMinutes"`
}

type TopArtist struct {
	ArtistID     string  `db:"artist_id"     json:"artistId"`
	Name         string  `db:"name"          json:"name"`
	PlayCount    int64   `db:"play_count"    json:"playCount"`
	TotalMinutes float64 `db:"total_minutes" json:"totalMinutes"`
}

type ListenSummary struct {
	PlayCount     int64   `db:"play_count"     json:"playCount"`
	TotalMinutes  float64 `db:"total_minutes"  json:"totalMinutes"`
	UniqueSongs   int64   `db:"unique_songs"   json:"uniqueSongs"`
	UniqueArtists int64   `db:"unique_artists" json:"uniqueArtists"`
}

// TasteProfile is the average of the VibeNet audio-feature tags (acousticness,
// danceability, energy, instrumentalness, liveness, speechiness, valence)
// across every scrobbled track in the period. A nil field means none of the
// scrobbled tracks had that tag set - untagged tracks are skipped for that
// field's average, not treated as 0. TrackCount is the number of scrobbles
// the averages were computed over (same population as
// ListenSummary.PlayCount for the same period) - individual fields may be
// averaged over fewer tracks than TrackCount if some weren't tagged.
type TasteProfile struct {
	Acousticness     *float64 `db:"acousticness"     json:"acousticness,omitempty"`
	Danceability     *float64 `db:"danceability"     json:"danceability,omitempty"`
	Energy           *float64 `db:"energy"           json:"energy,omitempty"`
	Instrumentalness *float64 `db:"instrumentalness" json:"instrumentalness,omitempty"`
	Liveness         *float64 `db:"liveness"         json:"liveness,omitempty"`
	Speechiness      *float64 `db:"speechiness"      json:"speechiness,omitempty"`
	Valence          *float64 `db:"valence"          json:"valence,omitempty"`
	TrackCount       int64    `db:"track_count"      json:"trackCount"`
}
