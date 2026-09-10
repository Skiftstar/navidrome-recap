package subsonic

import (
	"time"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetRecap", func() {
	var router *Router
	var ds *tests.MockDataStore
	var repo *tests.MockScrobbleRepo

	BeforeEach(func() {
		repo = &tests.MockScrobbleRepo{}
		ds = &tests.MockDataStore{MockedScrobble: repo}
		router = New(ds, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	})

	It("resolves ?year= to Jan 1 - Dec 31 UTC and combines summary/topSongs/topArtists/tasteProfile into one payload", func() {
		energy := 0.8
		repo.SummaryResult = model.ListenSummary{PlayCount: 4, TotalMinutes: 13.3, UniqueSongs: 3, UniqueArtists: 3}
		repo.TopSongsResult = []model.TopSong{{MediaFileID: "s1", Title: "Song 1", Artist: "Solo Artist", PlayCount: 2, TotalMinutes: 6.7}}
		repo.TopArtistsResult = []model.TopArtist{{ArtistID: "a1", Name: "Solo Artist", PlayCount: 3, TotalMinutes: 8.3}}
		repo.TasteProfileResult = model.TasteProfile{Energy: &energy, TrackCount: 4}

		r := newGetRequest("year=2024")
		resp, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastFrom).To(Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
		Expect(repo.LastTo).To(Equal(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)))
		Expect(repo.LastLimit).To(Equal(defaultRecapLimit))

		Expect(resp.Recap).ToNot(BeNil())
		Expect(resp.Recap.Summary.PlayCount).To(Equal(int64(4)))
		Expect(resp.Recap.Summary.UniqueArtists).To(Equal(int64(3)))
		Expect(resp.Recap.TopSongs).To(HaveLen(1))
		Expect(resp.Recap.TopSongs[0].MediaFileId).To(Equal("s1"))
		Expect(resp.Recap.TopArtists).To(HaveLen(1))
		Expect(resp.Recap.TopArtists[0].ArtistId).To(Equal("a1"))
		Expect(resp.Recap.TasteProfile.Energy).To(Equal(&energy))
		Expect(resp.Recap.TasteProfile.Danceability).To(BeNil())
	})

	It("honors a count param, capped at maxRecapLimit", func() {
		r := newGetRequest("year=2024", "count=500")
		_, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastLimit).To(Equal(maxRecapLimit))
	})

	It("defaults to the current year when no year is given", func() {
		r := newGetRequest()
		_, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastFrom.Year()).To(Equal(time.Now().UTC().Year()))
	})
})
