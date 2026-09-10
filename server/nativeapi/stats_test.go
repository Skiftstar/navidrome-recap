package nativeapi

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"time"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/request"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("Stats/Recap Endpoints", func() {
	var (
		ds       *tests.MockDataStore
		repo     *tests.MockScrobbleRepo
		user     model.User
		userRepo *tests.MockedUserRepo
	)

	BeforeEach(func() {
		repo = &tests.MockScrobbleRepo{}
		user = model.User{ID: "u1", UserName: "user"}
		userRepo = tests.CreateMockUserRepo()
		_ = userRepo.Put(&user)
		ds = &tests.MockDataStore{MockedScrobble: repo, MockedUser: userRepo, MockedProperty: &tests.MockedPropertyRepo{}}
	})

	newRequest := func(url string) *http.Request {
		req := httptest.NewRequest("GET", url, nil)
		return req.WithContext(request.WithUser(req.Context(), user))
	}

	Describe("GET /stats/summary", func() {
		It("resolves explicit from/to and returns the summary as JSON", func() {
			repo.SummaryResult = model.ListenSummary{PlayCount: 42, TotalMinutes: 123.5, UniqueSongs: 10, UniqueArtists: 5}

			req := newRequest("/stats/summary?from=2024-01-01T00:00:00Z&to=2024-12-31T23:59:59Z")
			w := httptest.NewRecorder()
			statsSummary(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(repo.LastFrom).To(Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
			Expect(repo.LastTo).To(Equal(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)))

			var got model.ListenSummary
			Expect(json.Unmarshal(w.Body.Bytes(), &got)).To(Succeed())
			Expect(got).To(Equal(repo.SummaryResult))
		})

		It("defaults to all-time (from zero to now) when no range is given", func() {
			before := time.Now().UTC()
			req := newRequest("/stats/summary")
			w := httptest.NewRecorder()
			statsSummary(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(repo.LastFrom.IsZero()).To(BeTrue())
			Expect(repo.LastTo).To(BeTemporally(">=", before))
			Expect(repo.LastTo).To(BeTemporally("<=", time.Now().UTC()))
		})
	})

	Describe("GET /stats/top-songs", func() {
		It("passes the limit query param through, capped at maxStatsLimit", func() {
			repo.TopSongsResult = []model.TopSong{{MediaFileID: "s1", Title: "Song", PlayCount: 3}}

			req := newRequest("/stats/top-songs?limit=500")
			w := httptest.NewRecorder()
			topSongs(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(repo.LastLimit).To(Equal(maxStatsLimit))

			var got []model.TopSong
			Expect(json.Unmarshal(w.Body.Bytes(), &got)).To(Succeed())
			Expect(got).To(Equal(repo.TopSongsResult))
		})

		It("defaults the limit when not given", func() {
			req := newRequest("/stats/top-songs")
			w := httptest.NewRecorder()
			topSongs(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(repo.LastLimit).To(Equal(defaultStatsLimit))
		})
	})

	Describe("GET /stats/top-artists", func() {
		It("returns the top artists as JSON", func() {
			repo.TopArtistsResult = []model.TopArtist{{ArtistID: "a1", Name: "Artist", PlayCount: 7}}

			req := newRequest("/stats/top-artists")
			w := httptest.NewRecorder()
			topArtists(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var got []model.TopArtist
			Expect(json.Unmarshal(w.Body.Bytes(), &got)).To(Succeed())
			Expect(got).To(Equal(repo.TopArtistsResult))
		})
	})

	Describe("GET /stats/taste-profile", func() {
		It("returns the taste profile as JSON", func() {
			energy := 0.8
			repo.TasteProfileResult = model.TasteProfile{Energy: &energy, TrackCount: 12}

			req := newRequest("/stats/taste-profile")
			w := httptest.NewRecorder()
			tasteProfile(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			var got model.TasteProfile
			Expect(json.Unmarshal(w.Body.Bytes(), &got)).To(Succeed())
			Expect(got).To(Equal(repo.TasteProfileResult))
		})
	})

	Describe("explicit from/to range", func() {
		It("resolves RFC3339 from/to", func() {
			req := newRequest("/stats/summary?from=2024-03-01T00:00:00Z&to=2024-03-31T23:59:59Z")
			w := httptest.NewRecorder()
			statsSummary(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(repo.LastFrom).To(Equal(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)))
			Expect(repo.LastTo).To(Equal(time.Date(2024, 3, 31, 23, 59, 59, 0, time.UTC)))
		})

		It("returns 400 for an unparsable from/to", func() {
			req := newRequest("/stats/summary?from=not-a-date")
			w := httptest.NewRecorder()
			statsSummary(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusBadRequest))
		})

		It("defaults an omitted 'to' to now, not the zero time (which would match nothing as an upper bound)", func() {
			before := time.Now().UTC()
			req := newRequest("/stats/summary?from=2024-01-01T00:00:00Z")
			w := httptest.NewRecorder()
			statsSummary(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(repo.LastFrom).To(Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
			Expect(repo.LastTo).To(BeTemporally(">=", before))
			Expect(repo.LastTo).To(BeTemporally("<=", time.Now().UTC()))
		})

		It("defaults an omitted 'from' to the zero time (unbounded start)", func() {
			req := newRequest("/stats/summary?to=2024-12-31T23:59:59Z")
			w := httptest.NewRecorder()
			statsSummary(ds)(w, req)

			Expect(w.Code).To(Equal(http.StatusOK))
			Expect(repo.LastFrom.IsZero()).To(BeTrue())
			Expect(repo.LastTo).To(Equal(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)))
		})
	})
})
