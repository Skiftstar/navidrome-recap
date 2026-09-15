package subsonic

import (
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/tests"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
)

var _ = Describe("GetVibeSimilarTracks", func() {
	var router *Router
	var ds *tests.MockDataStore
	var repo *tests.MockMediaFileRepo

	BeforeEach(func() {
		repo = tests.CreateMockMediaFileRepo()
		ds = &tests.MockDataStore{MockedMediaFile: repo}
		router = New(ds, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil, nil)
	})

	It("requires either id or all 7 vibe params", func() {
		r := newGetRequest()
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).To(HaveOccurred())
	})

	It("errors when only some of the 7 vibe params are given", func() {
		r := newGetRequest("acousticness=0.5", "danceability=0.5")
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).To(HaveOccurred())
	})

	It("resolves the profile from id, auto-excludes the seed, and returns full song info plus distance", func() {
		repo.GetVibeProfileResult = model.VibeProfile{Acousticness: 0.1, Danceability: 0.2, Energy: 0.3, Instrumentalness: 0.4, Liveness: 0.5, Speechiness: 0.6, Valence: 0.7}
		repo.GetVibeProfileOK = true
		repo.SimilarByVibeProfileResult = []model.VibeSimilarity{
			{MediaFile: model.MediaFile{ID: "s1", Title: "Song 1", Artist: "Artist 1"}, Distance: 0.05},
			{MediaFile: model.MediaFile{ID: "s2", Title: "Song 2", Artist: "Artist 2"}, Distance: 0.5},
		}

		r := newGetRequest("id=seed")
		resp, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.SimilarByVibeProfileLastProfile).To(Equal(repo.GetVibeProfileResult))
		Expect(repo.SimilarByVibeProfileLastExclude).To(ConsistOf("seed"))

		Expect(resp.VibeSimilarTracks).ToNot(BeNil())
		Expect(*resp.VibeSimilarTracks).To(HaveLen(2))
		Expect((*resp.VibeSimilarTracks)[0].Entry.Id).To(Equal("s1"))
		Expect((*resp.VibeSimilarTracks)[0].Entry.Title).To(Equal("Song 1"))
		Expect((*resp.VibeSimilarTracks)[0].Distance).To(Equal(0.05))
		Expect((*resp.VibeSimilarTracks)[1].Entry.Id).To(Equal("s2"))
		Expect((*resp.VibeSimilarTracks)[1].Distance).To(Equal(0.5))
	})

	It("returns an empty result, not an error, when the seed track has no VibeNet tags", func() {
		repo.GetVibeProfileOK = false

		r := newGetRequest("id=seed")
		resp, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(resp.VibeSimilarTracks).ToNot(BeNil())
		Expect(*resp.VibeSimilarTracks).To(BeEmpty())
	})

	It("builds the profile from the 7 raw vibe params when id is not given", func() {
		r := newGetRequest(
			"acousticness=0.1", "danceability=0.2", "energy=0.3", "instrumentalness=0.4",
			"liveness=0.5", "speechiness=0.6", "valence=0.7",
		)
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.SimilarByVibeProfileLastProfile).To(Equal(model.VibeProfile{
			Acousticness: 0.1, Danceability: 0.2, Energy: 0.3, Instrumentalness: 0.4,
			Liveness: 0.5, Speechiness: 0.6, Valence: 0.7,
		}))
		Expect(repo.SimilarByVibeProfileLastExclude).To(BeEmpty())
	})

	It("prefers id over raw vibe params when both are given", func() {
		repo.GetVibeProfileResult = model.VibeProfile{Acousticness: 0.9, Danceability: 0.9, Energy: 0.9, Instrumentalness: 0.9, Liveness: 0.9, Speechiness: 0.9, Valence: 0.9}
		repo.GetVibeProfileOK = true

		r := newGetRequest("id=seed",
			"acousticness=0.1", "danceability=0.2", "energy=0.3", "instrumentalness=0.4",
			"liveness=0.5", "speechiness=0.6", "valence=0.7",
		)
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.SimilarByVibeProfileLastProfile).To(Equal(repo.GetVibeProfileResult))
	})

	It("passes caller-supplied exclude IDs through, alongside the auto-excluded seed", func() {
		repo.GetVibeProfileOK = true

		r := newGetRequest("id=seed", "exclude=already-queued-1", "exclude=already-queued-2")
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.SimilarByVibeProfileLastExclude).To(ConsistOf("already-queued-1", "already-queued-2", "seed"))
	})

	It("caps an oversized count at maxVibeSimilarLimit", func() {
		r := newGetRequest("id=seed", "count=500")
		repo.GetVibeProfileOK = true
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.SimilarByVibeProfileLastCount).To(Equal(maxVibeSimilarLimit))
	})

	It("defaults the count when not given", func() {
		repo.GetVibeProfileOK = true
		r := newGetRequest("id=seed")
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.SimilarByVibeProfileLastCount).To(Equal(defaultVibeSimilarLimit))
	})
})
