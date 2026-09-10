package subsonic

import (
	"context"
	"strconv"
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

	It("resolves explicit from/to and combines summary/topSongs/topArtists/tasteProfile into one payload", func() {
		energy := 0.8
		repo.SummaryResult = model.ListenSummary{PlayCount: 4, TotalMinutes: 13.3, UniqueSongs: 3, UniqueArtists: 3}
		repo.TopSongsResult = []model.TopSong{{MediaFileID: "s1", Title: "Song 1", Artist: "Solo Artist", PlayCount: 2, TotalMinutes: 6.7}}
		repo.TopArtistsResult = []model.TopArtist{{ArtistID: "a1", Name: "Solo Artist", PlayCount: 3, TotalMinutes: 8.3}}
		repo.TasteProfileResult = model.TasteProfile{Energy: &energy, TrackCount: 4}

		mfRepo := ds.MediaFile(context.Background()).(*tests.MockMediaFileRepo)
		Expect(mfRepo.Put(&model.MediaFile{ID: "s1", Title: "Song 1", Artist: "Solo Artist", Album: "Album 1"})).To(Succeed())

		r := newGetRequest("from=2024-01-01T00:00:00Z", "to=2024-12-31T23:59:59Z")
		resp, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastFrom).To(Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
		Expect(repo.LastTo).To(Equal(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)))
		Expect(repo.LastLimit).To(Equal(defaultRecapLimit))

		Expect(resp.Recap).ToNot(BeNil())
		Expect(resp.Recap.Summary.PlayCount).To(Equal(int64(4)))
		Expect(resp.Recap.Summary.UniqueArtists).To(Equal(int64(3)))
		Expect(resp.Recap.TopSongs).To(HaveLen(1))
		Expect(resp.Recap.TopSongs[0].Entry.Id).To(Equal("s1"))
		Expect(resp.Recap.TopSongs[0].Entry.Title).To(Equal("Song 1"))
		Expect(resp.Recap.TopSongs[0].Entry.Album).To(Equal("Album 1"))
		Expect(resp.Recap.TopSongs[0].PlayCount).To(Equal(int64(2)))
		Expect(resp.Recap.TopSongs[0].TotalMinutes).To(Equal(6.7))
		Expect(resp.Recap.TopArtists).To(HaveLen(1))
		Expect(resp.Recap.TopArtists[0].ArtistId).To(Equal("a1"))
		Expect(resp.Recap.TasteProfile.Energy).To(Equal(&energy))
		Expect(resp.Recap.TasteProfile.Danceability).To(BeNil())
	})

	It("omits a top song whose MediaFile is no longer found (e.g. purged since the scrobble was recorded)", func() {
		repo.TopSongsResult = []model.TopSong{{MediaFileID: "missing", Title: "Gone", PlayCount: 1, TotalMinutes: 1}}

		r := newGetRequest()
		resp, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(resp.Recap.TopSongs).To(BeEmpty())
	})

	It("honors a count param, capped at maxRecapLimit", func() {
		r := newGetRequest("count=500")
		_, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastLimit).To(Equal(maxRecapLimit))
	})

	It("defaults to all-time (from zero to now) when no range is given", func() {
		before := time.Now().UTC()
		r := newGetRequest()
		_, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastFrom.IsZero()).To(BeTrue())
		Expect(repo.LastTo).To(BeTemporally(">=", before))
		Expect(repo.LastTo).To(BeTemporally("<=", time.Now().UTC()))
	})

	It("uses explicit from/to", func() {
		r := newGetRequest("from=2024-03-01T00:00:00Z", "to=2024-03-31T23:59:59Z")
		_, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastFrom).To(Equal(time.Date(2024, 3, 1, 0, 0, 0, 0, time.UTC)))
		Expect(repo.LastTo).To(Equal(time.Date(2024, 3, 31, 23, 59, 59, 0, time.UTC)))
	})

	It("accepts from/to as unix seconds", func() {
		from := time.Date(2024, 6, 1, 0, 0, 0, 0, time.UTC)
		to := time.Date(2024, 6, 30, 0, 0, 0, 0, time.UTC)
		r := newGetRequest(
			"from="+strconv.FormatInt(from.Unix(), 10),
			"to="+strconv.FormatInt(to.Unix(), 10),
		)
		_, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastFrom).To(Equal(from))
		Expect(repo.LastTo).To(Equal(to))
	})

	It("defaults an omitted 'to' to now, not the zero time (which would match nothing as an upper bound)", func() {
		before := time.Now().UTC()
		r := newGetRequest("from=2024-01-01T00:00:00Z")
		_, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastFrom).To(Equal(time.Date(2024, 1, 1, 0, 0, 0, 0, time.UTC)))
		Expect(repo.LastTo).To(BeTemporally(">=", before))
		Expect(repo.LastTo).To(BeTemporally("<=", time.Now().UTC()))
	})

	It("defaults an omitted 'from' to the zero time (unbounded start)", func() {
		r := newGetRequest("to=2024-12-31T23:59:59Z")
		_, err := router.GetRecap(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.LastFrom.IsZero()).To(BeTrue())
		Expect(repo.LastTo).To(Equal(time.Date(2024, 12, 31, 23, 59, 59, 0, time.UTC)))
	})

	It("returns an error for an unparsable from/to", func() {
		r := newGetRequest("from=not-a-date")
		_, err := router.GetRecap(r)

		Expect(err).To(HaveOccurred())
	})
})
