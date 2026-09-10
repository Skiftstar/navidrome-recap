package tests

import (
	"context"
	"strconv"
	"time"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
)

type MockScrobbleRepo struct {
	RecordedScrobbles []model.Scrobble
	ctx               context.Context

	// Canned results for the recap/stats aggregate methods below - set these
	// directly in a test to control what TopSongs/TopArtists/Summary/
	// TasteProfile return; they are not derived from RecordedScrobbles.
	TopSongsResult     []model.TopSong
	TopArtistsResult   []model.TopArtist
	SummaryResult      model.ListenSummary
	TasteProfileResult model.TasteProfile
	// LastFrom/LastTo record the range passed into the last stats call, so a
	// test can assert the request was parsed as expected.
	LastFrom, LastTo time.Time
	LastLimit        int
}

func (m *MockScrobbleRepo) Get(id string) (*model.Scrobble, error) {
	for idx := range m.RecordedScrobbles {
		if strconv.FormatInt(m.RecordedScrobbles[idx].ID, 10) == id {
			return &m.RecordedScrobbles[idx], nil
		}
	}

	return nil, model.ErrNotFound
}

func (m *MockScrobbleRepo) GetAll(options ...model.QueryOptions) (model.Scrobbles, error) {
	return m.RecordedScrobbles, nil
}

func (m *MockScrobbleRepo) CountAll(options ...model.QueryOptions) (int64, error) {
	return int64(len(m.RecordedScrobbles)), nil
}

func (m *MockScrobbleRepo) RecordScrobble(fileID string, submissionTime time.Time) error {
	user, _ := request.UserFrom(m.ctx)
	m.RecordedScrobbles = append(m.RecordedScrobbles, model.Scrobble{
		MediaFileID:    fileID,
		UserID:         user.ID,
		SubmissionTime: submissionTime.Unix(),
	})
	return nil
}

// TopSongs, TopArtists, Summary and TasteProfile return the canned
// *Result fields above (not derived from RecordedScrobbles), and record the
// range/limit they were called with in Last{From,To,Limit}.
func (m *MockScrobbleRepo) TopSongs(from, to time.Time, limit int) ([]model.TopSong, error) {
	m.LastFrom, m.LastTo, m.LastLimit = from, to, limit
	return m.TopSongsResult, nil
}

func (m *MockScrobbleRepo) TopArtists(from, to time.Time, limit int) ([]model.TopArtist, error) {
	m.LastFrom, m.LastTo, m.LastLimit = from, to, limit
	return m.TopArtistsResult, nil
}

func (m *MockScrobbleRepo) Summary(from, to time.Time) (model.ListenSummary, error) {
	m.LastFrom, m.LastTo = from, to
	return m.SummaryResult, nil
}

func (m *MockScrobbleRepo) TasteProfile(from, to time.Time) (model.TasteProfile, error) {
	m.LastFrom, m.LastTo = from, to
	return m.TasteProfileResult, nil
}

var _ model.ScrobbleRepository = (*MockScrobbleRepo)(nil)
