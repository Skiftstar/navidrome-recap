package persistence

import (
	"context"
	"time"

	"github.com/Masterminds/squirrel"
	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/model/id"
	"github.com/navidrome/navidrome/model/request"
	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"
	"github.com/pocketbase/dbx"
)

var _ = Describe("ScrobbleRepository", func() {
	var repo model.ScrobbleRepository
	var ctx context.Context

	Describe("RecordScrobble", func() {
		var fileID string
		var userID string
		var rawRepo sqlRepository

		BeforeEach(func() {
			fileID = id.NewRandom()
			userID = id.NewRandom()
			ctx = request.WithUser(log.NewContext(GinkgoT().Context()), model.User{ID: userID, UserName: "johndoe", IsAdmin: true})
			db := GetDBXBuilder()
			repo = NewScrobbleRepository(ctx, db)

			rawRepo = sqlRepository{
				ctx:       ctx,
				tableName: "scrobbles",
				db:        db,
			}
		})

		AfterEach(func() {
			_, _ = rawRepo.db.Delete("scrobbles", dbx.HashExp{"media_file_id": fileID}).Execute()
			_, _ = rawRepo.db.Delete("media_file", dbx.HashExp{"id": fileID}).Execute()
			_, _ = rawRepo.db.Delete("user", dbx.HashExp{"id": userID}).Execute()
		})

		It("records a scrobble event", func() {
			submissionTime := time.Now().UTC()

			// Insert User
			_, err := rawRepo.db.Insert("user", dbx.Params{
				"id":         userID,
				"user_name":  "user",
				"password":   "pw",
				"created_at": time.Now(),
				"updated_at": time.Now(),
			}).Execute()
			Expect(err).ToNot(HaveOccurred())

			// Insert MediaFile
			_, err = rawRepo.db.Insert("media_file", dbx.Params{
				"id":         fileID,
				"path":       "path",
				"created_at": time.Now(),
				"updated_at": time.Now(),
			}).Execute()
			Expect(err).ToNot(HaveOccurred())

			err = repo.RecordScrobble(fileID, submissionTime, nil)
			Expect(err).ToNot(HaveOccurred())

			// Verify insertion
			var scrobble struct {
				MediaFileID      string `db:"media_file_id"`
				UserID           string `db:"user_id"`
				SubmissionTime   int64  `db:"submission_time"`
				PlayedDurationMs *int64 `db:"played_duration_ms"`
			}
			err = rawRepo.db.Select("*").From("scrobbles").
				Where(dbx.HashExp{"media_file_id": fileID, "user_id": userID}).
				One(&scrobble)
			Expect(err).ToNot(HaveOccurred())
			Expect(scrobble.MediaFileID).To(Equal(fileID))
			Expect(scrobble.UserID).To(Equal(userID))
			Expect(scrobble.SubmissionTime).To(Equal(submissionTime.Unix()))
			Expect(scrobble.PlayedDurationMs).To(BeNil())
		})

		It("persists a real played duration when supplied", func() {
			submissionTime := time.Now().UTC()

			_, err := rawRepo.db.Insert("user", dbx.Params{
				"id": userID, "user_name": "user", "password": "pw",
				"created_at": time.Now(), "updated_at": time.Now(),
			}).Execute()
			Expect(err).ToNot(HaveOccurred())

			_, err = rawRepo.db.Insert("media_file", dbx.Params{
				"id": fileID, "path": "path",
				"created_at": time.Now(), "updated_at": time.Now(),
			}).Execute()
			Expect(err).ToNot(HaveOccurred())

			playedMs := int64(123_456)
			err = repo.RecordScrobble(fileID, submissionTime, &playedMs)
			Expect(err).ToNot(HaveOccurred())

			var scrobble struct {
				PlayedDurationMs *int64 `db:"played_duration_ms"`
			}
			err = rawRepo.db.Select("played_duration_ms").From("scrobbles").
				Where(dbx.HashExp{"media_file_id": fileID, "user_id": userID}).
				One(&scrobble)
			Expect(err).ToNot(HaveOccurred())
			Expect(scrobble.PlayedDurationMs).ToNot(BeNil())
			Expect(*scrobble.PlayedDurationMs).To(Equal(playedMs))
		})
	})

	Describe("Recap stats (TopSongs/TopArtists/Summary/TasteProfile)", func() {
		var userID, soloArtistID, artistAID, artistBID string
		var soloSongID, duoSongID, untaggedDuoSongID string
		var rawRepo sqlRepository
		var inRange, outOfRange time.Time

		BeforeEach(func() {
			userID = id.NewRandom()
			soloArtistID = id.NewRandom()
			artistAID = id.NewRandom()
			artistBID = id.NewRandom()
			soloSongID = id.NewRandom()        // solo artist, tagged (acousticness+energy), duration 200s
			duoSongID = id.NewRandom()         // solo artist, tagged (acousticness only), duration 100s
			untaggedDuoSongID = id.NewRandom() // two credited artists, untagged, duration 300s

			ctx = request.WithUser(log.NewContext(GinkgoT().Context()), model.User{ID: userID, UserName: "recapuser", IsAdmin: true})
			db := GetDBXBuilder()
			repo = NewScrobbleRepository(ctx, db)
			rawRepo = sqlRepository{ctx: ctx, tableName: "scrobbles", db: db}

			inRange = time.Date(2024, 6, 15, 12, 0, 0, 0, time.UTC)
			outOfRange = time.Date(2023, 6, 15, 12, 0, 0, 0, time.UTC)

			_, err := db.Insert("user", dbx.Params{
				"id": userID, "user_name": "recapuser", "password": "pw",
				"created_at": time.Now(), "updated_at": time.Now(),
			}).Execute()
			Expect(err).ToNot(HaveOccurred())

			for _, a := range []struct{ id, name string }{
				{soloArtistID, "Solo Artist"}, {artistAID, "Artist A"}, {artistBID, "Artist B"},
			} {
				_, err := db.Insert("artist", dbx.Params{"id": a.id, "name": a.name}).Execute()
				Expect(err).ToNot(HaveOccurred())
			}

			insertSong := func(id, title, artist, artistID string, duration int, tags model.Tags) {
				if tags == nil {
					tags = model.Tags{}
				}
				_, err := db.Insert("media_file", dbx.Params{
					"id": id, "path": id, "title": title, "artist": artist, "artist_id": artistID,
					"duration": duration, "tags": marshalTags(tags),
					"created_at": time.Now(), "updated_at": time.Now(),
				}).Execute()
				Expect(err).ToNot(HaveOccurred())
			}
			insertSong(soloSongID, "Solo Song", "Solo Artist", soloArtistID, 200, model.Tags{
				"acousticness": {"0.2"}, "energy": {"0.8"},
			})
			insertSong(duoSongID, "Another Solo Song", "Solo Artist", soloArtistID, 100, model.Tags{
				"acousticness": {"0.6"},
			})
			insertSong(untaggedDuoSongID, "Duet Song", "Artist A", artistAID, 300, nil)

			insertParticipant := func(mediaFileID, artistID string) {
				_, err := db.Insert("media_file_artists", dbx.Params{
					"media_file_id": mediaFileID, "artist_id": artistID, "role": model.RoleArtist.String(),
				}).Execute()
				Expect(err).ToNot(HaveOccurred())
			}
			insertParticipant(soloSongID, soloArtistID)
			insertParticipant(duoSongID, soloArtistID)
			insertParticipant(untaggedDuoSongID, artistAID)
			insertParticipant(untaggedDuoSongID, artistBID)

			// soloSong scrobbled twice in-range (+ once out-of-range, to verify date filtering),
			// duoSong once in-range, the duet song once in-range. soloSong's first
			// in-range scrobble carries a real played duration (90s of its 200s length,
			// as if the listener skipped partway through) to exercise the mixed
			// real-duration/full-duration-fallback math; every other scrobble has no
			// played duration, falling back to the track's full length.
			realPlayedMs := int64(90_000)
			Expect(repo.RecordScrobble(soloSongID, inRange, &realPlayedMs)).To(Succeed())
			Expect(repo.RecordScrobble(soloSongID, inRange.Add(time.Hour), nil)).To(Succeed())
			Expect(repo.RecordScrobble(soloSongID, outOfRange, nil)).To(Succeed())
			Expect(repo.RecordScrobble(duoSongID, inRange, nil)).To(Succeed())
			Expect(repo.RecordScrobble(untaggedDuoSongID, inRange, nil)).To(Succeed())
		})

		AfterEach(func() {
			for _, songID := range []string{soloSongID, duoSongID, untaggedDuoSongID} {
				_, _ = rawRepo.db.Delete("scrobbles", dbx.HashExp{"media_file_id": songID}).Execute()
				_, _ = rawRepo.db.Delete("media_file_artists", dbx.HashExp{"media_file_id": songID}).Execute()
				_, _ = rawRepo.db.Delete("media_file", dbx.HashExp{"id": songID}).Execute()
			}
			for _, artistID := range []string{soloArtistID, artistAID, artistBID} {
				_, _ = rawRepo.db.Delete("artist", dbx.HashExp{"id": artistID}).Execute()
			}
			_, _ = rawRepo.db.Delete("user", dbx.HashExp{"id": userID}).Execute()
		})

		yearRange := func(year int) (time.Time, time.Time) {
			return time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC),
				time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Second)
		}

		Describe("TopSongs", func() {
			It("orders by play count and computes minutes from the real played duration where known, falling back to full track duration otherwise", func() {
				from, to := yearRange(2024)
				songs, err := repo.TopSongs(from, to, 10)
				Expect(err).ToNot(HaveOccurred())
				Expect(songs).To(HaveLen(3))

				byID := map[string]model.TopSong{}
				for _, s := range songs {
					byID[s.MediaFileID] = s
				}
				Expect(songs[0].MediaFileID).To(Equal(soloSongID), "the twice-scrobbled song should be first")
				Expect(byID[soloSongID].PlayCount).To(Equal(int64(2)))
				// 90s (real, first scrobble) + 200s (fallback, second scrobble) = 290s
				Expect(byID[soloSongID].TotalMinutes).To(BeNumerically("~", 290.0/60.0, 0.001))
				Expect(byID[duoSongID].PlayCount).To(Equal(int64(1)))
				Expect(byID[duoSongID].TotalMinutes).To(BeNumerically("~", 100.0/60.0, 0.001))
				Expect(byID[untaggedDuoSongID].PlayCount).To(Equal(int64(1)))
				Expect(byID[untaggedDuoSongID].TotalMinutes).To(BeNumerically("~", 300.0/60.0, 0.001))
			})
		})

		Describe("TopArtists", func() {
			It("attributes each credited artist their own play count, without inflating a solo artist's count", func() {
				from, to := yearRange(2024)
				artists, err := repo.TopArtists(from, to, 10)
				Expect(err).ToNot(HaveOccurred())
				Expect(artists).To(HaveLen(3))

				byID := map[string]model.TopArtist{}
				for _, a := range artists {
					byID[a.ArtistID] = a
				}
				Expect(byID[soloArtistID].PlayCount).To(Equal(int64(3)), "2 plays of soloSong + 1 of duoSong")
				// 290s (soloSong, real+fallback mix) + 100s (duoSong, fallback) = 390s
				Expect(byID[soloArtistID].TotalMinutes).To(BeNumerically("~", 390.0/60.0, 0.001))
				Expect(byID[artistAID].PlayCount).To(Equal(int64(1)))
				Expect(byID[artistBID].PlayCount).To(Equal(int64(1)))
			})
		})

		Describe("Summary", func() {
			It("does not double-count a multi-artist track's play in the overall totals", func() {
				from, to := yearRange(2024)
				summary, err := repo.Summary(from, to)
				Expect(err).ToNot(HaveOccurred())

				// 2 (soloSong) + 1 (duoSong) + 1 (untaggedDuoSong) = 4 - NOT 5, even though
				// untaggedDuoSong has 2 credited artists (that's the bug TopArtists's join
				// would cause here if Summary reused it for the overall totals).
				Expect(summary.PlayCount).To(Equal(int64(4)))
				// 290s (soloSong, real+fallback mix) + 100s (duoSong, fallback) + 300s (untaggedDuoSong, fallback) = 690s
				Expect(summary.TotalMinutes).To(BeNumerically("~", 690.0/60.0, 0.001))
				Expect(summary.UniqueSongs).To(Equal(int64(3)))
				Expect(summary.UniqueArtists).To(Equal(int64(3)))
			})

			It("returns zeroes, not an error, for a range with no scrobbles", func() {
				from, to := yearRange(1999)
				summary, err := repo.Summary(from, to)
				Expect(err).ToNot(HaveOccurred())
				Expect(summary).To(Equal(model.ListenSummary{}))
			})
		})

		Describe("TasteProfile", func() {
			It("averages only over tracks that have each tag, ignoring untagged tracks", func() {
				from, to := yearRange(2024)
				profile, err := repo.TasteProfile(from, to)
				Expect(err).ToNot(HaveOccurred())

				// acousticness: soloSong (0.2 x2 scrobbles) + duoSong (0.6 x1) = (0.2+0.2+0.6)/3
				Expect(profile.Acousticness).ToNot(BeNil())
				Expect(*profile.Acousticness).To(BeNumerically("~", (0.2+0.2+0.6)/3.0, 0.001))
				// energy: only soloSong has it, both its scrobbles = 0.8
				Expect(profile.Energy).ToNot(BeNil())
				Expect(*profile.Energy).To(BeNumerically("~", 0.8, 0.001))
				// no scrobbled track has danceability set
				Expect(profile.Danceability).To(BeNil())
				Expect(profile.TrackCount).To(Equal(int64(4)))
			})

			It("returns all-nil fields, not an error, for a range with no scrobbles", func() {
				from, to := yearRange(1999)
				profile, err := repo.TasteProfile(from, to)
				Expect(err).ToNot(HaveOccurred())
				Expect(profile).To(Equal(model.TasteProfile{}))
			})
		})
	})

	Context("admin user (id userid)", func() {
		BeforeEach(func() {
			ctx = request.WithUser(log.NewContext(context.TODO()), adminUser)
			repo = NewScrobbleRepository(ctx, GetDBXBuilder())
		})

		Describe("Count", func() {
			It("Returns the number of scrobbles in the DB for admin user", func() {
				Expect(repo.CountAll()).To(Equal(int64(2)))
			})

			It("returns scrobbles in a range", func() {
				Expect(repo.CountAll(model.QueryOptions{Filters: squirrel.LtOrEq{"submission_time": 1}})).To(Equal(int64(1)))
			})
		})

		Describe("Get", func() {
			It("returns an existing scrobble for the user", func() {
				scrobble, err := repo.Get("1")
				Expect(err).To(BeNil())
				Expect(scrobble.ID).To(Equal(int64(1)))
				Expect(scrobble.MediaFileID).To(Equal("1001"))
				Expect(scrobble.SubmissionTime).To(Equal(firstScrobble.SubmissionTime))

			})

			It("does not return a scrobble that exists for another user", func() {
				_, err := repo.Get("2")
				Expect(err).To(MatchError(model.ErrNotFound))
			})

			It("does not return a scrobble that does not exist", func() {
				_, err := repo.Get("444")
				Expect(err).To(MatchError(model.ErrNotFound))
			})
		})

		Describe("GetAll", func() {
			It("returns all scrobbles in reverse order", func() {
				scrobbles, err := repo.GetAll(model.QueryOptions{
					Sort:  "submission_time",
					Order: "DESC",
				})
				Expect(err).To(BeNil())
				Expect(scrobbles).To(HaveLen(2))

				Expect(scrobbles[0].ID).To(Equal(int64(3)))
				Expect(scrobbles[0].MediaFileID).To(Equal("1002"))
				Expect(scrobbles[0].SubmissionTime).To(Equal(thirdScrobble.SubmissionTime))

				Expect(scrobbles[1].ID).To(Equal(int64(1)))
				Expect(scrobbles[1].MediaFileID).To(Equal("1001"))
				Expect(scrobbles[1].SubmissionTime).To(Equal(firstScrobble.SubmissionTime))
			})

			It("returns scrobbles in a range", func() {
				scrobbles, err := repo.GetAll(model.QueryOptions{
					Filters: squirrel.GtOrEq{"submission_time": 1}})

				Expect(err).To(BeNil())
				Expect(scrobbles).To(HaveLen(1))

				Expect(scrobbles[0].ID).To(Equal(int64(3)))
				Expect(scrobbles[0].MediaFileID).To(Equal("1002"))
				Expect(scrobbles[0].SubmissionTime).To(Equal(thirdScrobble.SubmissionTime))
			})
		})
	})

	Context("non-admin user", func() {
		BeforeEach(func() {
			ctx = request.WithUser(log.NewContext(context.TODO()), regularUser)
			repo = NewScrobbleRepository(ctx, GetDBXBuilder())
		})

		Describe("Count", func() {
			It("Returns the number of scrobbles in the DB for admin user", func() {
				Expect(repo.CountAll()).To(Equal(int64(1)))
			})

			It("returns scrobbles in a range", func() {
				Expect(repo.CountAll(model.QueryOptions{Filters: squirrel.LtOrEq{"submission_time": 1}})).To(Equal(int64(0)))
			})
		})

		Describe("Get", func() {
			It("returns an existing scrobble for the user", func() {
				scrobble, err := repo.Get("2")
				Expect(err).To(BeNil())
				Expect(scrobble.ID).To(Equal(int64(2)))
				Expect(scrobble.MediaFileID).To(Equal("1003"))
				Expect(scrobble.SubmissionTime).To(Equal(secondScrobble.SubmissionTime))
			})

			It("does not return a scrobble that exists for another user", func() {
				_, err := repo.Get("1")
				Expect(err).To(MatchError(model.ErrNotFound))
			})

			It("does not return a scrobble that does not exist", func() {
				_, err := repo.Get("444")
				Expect(err).To(MatchError(model.ErrNotFound))
			})
		})

		Describe("GetAll", func() {
			It("returns all scrobbles in reverse order", func() {
				scrobbles, err := repo.GetAll(model.QueryOptions{
					Sort:  "submission_time",
					Order: "DESC",
				})
				Expect(err).To(BeNil())
				Expect(scrobbles).To(HaveLen(1))

				Expect(scrobbles[0].ID).To(Equal(int64(2)))
				Expect(scrobbles[0].MediaFileID).To(Equal("1003"))
				Expect(scrobbles[0].SubmissionTime).To(Equal(secondScrobble.SubmissionTime))
			})

			It("returns scrobbles in a range", func() {
				scrobbles, err := repo.GetAll(model.QueryOptions{
					Filters: squirrel.GtOrEq{"submission_time": 1}})

				Expect(err).To(BeNil())
				Expect(scrobbles).To(HaveLen(1))

				Expect(scrobbles[0].ID).To(Equal(int64(2)))
				Expect(scrobbles[0].MediaFileID).To(Equal("1003"))
				Expect(scrobbles[0].SubmissionTime).To(Equal(secondScrobble.SubmissionTime))
			})
		})
	})
})
