package persistence

import (
	"context"
	"math"
	"strconv"
	"time"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pocketbase/dbx"
)

var _ = Describe("MediaFileRepository VibeNet similarity", func() {
	var repo model.MediaFileRepository
	var ctx context.Context
	var userID string
	var seedID, closeID, farID, partialID, noTagsID string
	var rawRepo sqlRepository

	BeforeEach(func() {
		userID = id.NewRandom()
		seedID = id.NewRandom()
		closeID = id.NewRandom()
		farID = id.NewRandom()
		partialID = id.NewRandom()
		noTagsID = id.NewRandom()

		ctx = request.WithUser(log.NewContext(GinkgoT().Context()), model.User{ID: userID, UserName: "vibeuser", IsAdmin: true})
		db := GetDBXBuilder()
		repo = NewMediaFileRepository(ctx, db)
		rawRepo = sqlRepository{ctx: ctx, tableName: "media_file", db: db}

		insertSong := func(mfID string, tags model.Tags) {
			if tags == nil {
				tags = model.Tags{}
			}
			_, err := db.Insert("media_file", dbx.Params{
				"id": mfID, "path": mfID, "title": mfID, "artist": "Artist", "library_id": 1,
				"duration": 200, "tags": marshalTags(tags),
				"created_at": time.Now(), "updated_at": time.Now(),
			}).Execute()
			Expect(err).ToNot(HaveOccurred())
		}

		vec := func(v float64) model.Tags {
			return model.Tags{
				"acousticness": {ff(v)}, "danceability": {ff(v)}, "energy": {ff(v)},
				"instrumentalness": {ff(v)}, "liveness": {ff(v)}, "speechiness": {ff(v)}, "valence": {ff(v)},
			}
		}

		insertSong(seedID, vec(0.5))
		insertSong(closeID, vec(0.51))
		insertSong(farID, vec(0.9))
		// partial: missing "valence" - must be excluded as a candidate
		partial := vec(0.51)
		delete(partial, "valence")
		insertSong(partialID, partial)
		insertSong(noTagsID, nil)
	})

	AfterEach(func() {
		for _, mfID := range []string{seedID, closeID, farID, partialID, noTagsID} {
			_, _ = rawRepo.db.Delete("media_file", dbx.HashExp{"id": mfID}).Execute()
		}
	})

	Describe("GetVibeProfile", func() {
		It("returns the track's profile", func() {
			profile, ok, err := repo.GetVibeProfile(seedID)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeTrue())
			Expect(profile).To(Equal(profileOf(0.5)))
		})

		It("returns ok=false, not an error, for a track with no VibeNet tags", func() {
			_, ok, err := repo.GetVibeProfile(noTagsID)
			Expect(err).ToNot(HaveOccurred())
			Expect(ok).To(BeFalse())
		})
	})

	Describe("SimilarByVibeProfile", func() {
		It("orders candidates by distance, excluding the seed itself and any candidate missing a VibeNet tag", func() {
			results, err := repo.SimilarByVibeProfile(profileOf(0.5), []string{seedID}, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(2))

			Expect(results[0].MediaFile.ID).To(Equal(closeID))
			Expect(results[0].Distance).To(BeNumerically("~", math.Sqrt(7*0.01*0.01), 1e-9))
			Expect(results[1].MediaFile.ID).To(Equal(farID))
			Expect(results[1].Distance).To(BeNumerically("~", math.Sqrt(7*0.4*0.4), 1e-9))
		})

		It("truncates to the requested count", func() {
			results, err := repo.SimilarByVibeProfile(profileOf(0.5), []string{seedID}, 1)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].MediaFile.ID).To(Equal(closeID))
		})

		It("searches by an explicit profile with no seed track excluded", func() {
			results, err := repo.SimilarByVibeProfile(profileOf(0.5), nil, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(3))

			Expect(results[0].MediaFile.ID).To(Equal(seedID))
			Expect(results[0].Distance).To(BeNumerically("~", 0, 1e-9))
			Expect(results[1].MediaFile.ID).To(Equal(closeID))
			Expect(results[2].MediaFile.ID).To(Equal(farID))
		})

		It("omits excluded IDs even when they'd otherwise be the closest match", func() {
			results, err := repo.SimilarByVibeProfile(profileOf(0.5), []string{seedID, closeID}, 10)
			Expect(err).ToNot(HaveOccurred())
			Expect(results).To(HaveLen(1))
			Expect(results[0].MediaFile.ID).To(Equal(farID))
		})
	})
})

// ff formats a float the same way the scanner/config would produce for a
// VibeNet tag value - a plain decimal string.
func ff(v float64) string {
	return strconv.FormatFloat(v, 'f', -1, 64)
}

// profileOf builds a model.VibeProfile with the same value in all 7 dimensions.
func profileOf(v float64) model.VibeProfile {
	return model.VibeProfile{
		Acousticness: v, Danceability: v, Energy: v, Instrumentalness: v,
		Liveness: v, Speechiness: v, Valence: v,
	}
}
