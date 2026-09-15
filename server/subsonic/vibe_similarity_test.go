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

	It("requires the id parameter", func() {
		r := newGetRequest()
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).To(HaveOccurred())
	})

	It("returns the full song info plus distance for each match", func() {
		repo.SimilarByVibeResult = []model.VibeSimilarity{
			{MediaFile: model.MediaFile{ID: "s1", Title: "Song 1", Artist: "Artist 1"}, Distance: 0.05},
			{MediaFile: model.MediaFile{ID: "s2", Title: "Song 2", Artist: "Artist 2"}, Distance: 0.5},
		}

		r := newGetRequest("id=seed")
		resp, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(resp.VibeSimilarTracks).ToNot(BeNil())
		Expect(*resp.VibeSimilarTracks).To(HaveLen(2))
		Expect((*resp.VibeSimilarTracks)[0].Entry.Id).To(Equal("s1"))
		Expect((*resp.VibeSimilarTracks)[0].Entry.Title).To(Equal("Song 1"))
		Expect((*resp.VibeSimilarTracks)[0].Distance).To(Equal(0.05))
		Expect((*resp.VibeSimilarTracks)[1].Entry.Id).To(Equal("s2"))
		Expect((*resp.VibeSimilarTracks)[1].Distance).To(Equal(0.5))
	})

	It("caps an oversized count at maxVibeSimilarLimit", func() {
		r := newGetRequest("id=seed", "count=500")
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.SimilarByVibeLastCount).To(Equal(maxVibeSimilarLimit))
	})

	It("defaults the count when not given", func() {
		r := newGetRequest("id=seed")
		_, err := router.GetVibeSimilarTracks(r)

		Expect(err).ToNot(HaveOccurred())
		Expect(repo.SimilarByVibeLastCount).To(Equal(defaultVibeSimilarLimit))
	})
})
