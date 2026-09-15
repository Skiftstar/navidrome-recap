package subsonic

import (
	"fmt"
	"net/http"
	"strconv"

	"github.com/navidrome/navidrome/model"
	"github.com/navidrome/navidrome/server/subsonic/responses"
	"github.com/navidrome/navidrome/utils/req"
)

const (
	defaultVibeSimilarLimit = 20
	maxVibeSimilarLimit     = 100
)

// GetVibeSimilarTracks is a fork-only, non-standard Subsonic extension (no
// such endpoint exists in the real Subsonic/OpenSubsonic spec - same idea as
// getSonicSimilarTracks/findSonicPath in sonic_similarity.go) that returns
// tracks similar to a VibeNet audio-feature profile, computed entirely from
// Navidrome's own data (see model.MediaFileRepository.SimilarByVibeProfile).
//
// The profile comes from either:
//   - 'id': the seed track's own profile (via GetVibeProfile) - the seed
//     itself is always added to 'exclude' automatically. Takes priority if
//     both 'id' and the raw vibe params below are given.
//   - all 7 raw vibe params (acousticness, danceability, energy,
//     instrumentalness, liveness, speechiness, valence) - lets a caller
//     search from an arbitrary profile, e.g. the averaged profile of a
//     playlist/queue, without picking one track as the seed.
//
// 'exclude' (repeatable, like this endpoint family's 'id' param elsewhere -
// see Scrobble in media_annotation.go) lists track IDs to omit from the
// results regardless of distance - e.g. tracks already queued or skipped.
func (api *Router) GetVibeSimilarTracks(r *http.Request) (*responses.Subsonic, error) {
	ctx := r.Context()
	p := req.Params(r)
	exclude := p.Strings("exclude")

	var profile model.VibeProfile
	if id, idErr := p.String("id"); idErr == nil {
		prof, ok, err := api.ds.MediaFile(ctx).GetVibeProfile(id)
		if err != nil {
			return nil, err
		}
		if !ok {
			response := newResponse()
			response.VibeSimilarTracks = &responses.Array[responses.VibeSimilarMatch]{}
			return response, nil
		}
		profile = prof
		exclude = append(exclude, id)
	} else {
		var err error
		profile, err = parseVibeProfileParams(p)
		if err != nil {
			return nil, newError(responses.ErrorMissingParameter, "%s", err)
		}
	}

	count := p.IntOr("count", defaultVibeSimilarLimit)
	if count <= 0 {
		count = defaultVibeSimilarLimit
	}
	count = min(count, maxVibeSimilarLimit)

	results, err := api.ds.MediaFile(ctx).SimilarByVibeProfile(profile, exclude, count)
	if err != nil {
		return nil, err
	}

	resp := make(responses.Array[responses.VibeSimilarMatch], len(results))
	for i, res := range results {
		resp[i] = responses.VibeSimilarMatch{
			Entry:    childFromMediaFile(ctx, res.MediaFile),
			Distance: res.Distance,
		}
	}
	response := newResponse()
	response.VibeSimilarTracks = &resp
	return response, nil
}

// parseVibeProfileParams reads all 7 vibe params, required together - a
// missing one would silently skew the distance metric rather than being an
// obvious error, so this fails loudly instead of defaulting it to 0.
func parseVibeProfileParams(p *req.Values) (model.VibeProfile, error) {
	get := func(name string) (float64, error) {
		s := p.StringOr(name, "")
		if s == "" {
			return 0, fmt.Errorf("missing parameter: '%s' (required when 'id' is not given)", name)
		}
		v, err := strconv.ParseFloat(s, 64)
		if err != nil {
			return 0, fmt.Errorf("invalid parameter: '%s' must be a number", name)
		}
		return v, nil
	}
	var profile model.VibeProfile
	var err error
	if profile.Acousticness, err = get("acousticness"); err != nil {
		return profile, err
	}
	if profile.Danceability, err = get("danceability"); err != nil {
		return profile, err
	}
	if profile.Energy, err = get("energy"); err != nil {
		return profile, err
	}
	if profile.Instrumentalness, err = get("instrumentalness"); err != nil {
		return profile, err
	}
	if profile.Liveness, err = get("liveness"); err != nil {
		return profile, err
	}
	if profile.Speechiness, err = get("speechiness"); err != nil {
		return profile, err
	}
	if profile.Valence, err = get("valence"); err != nil {
		return profile, err
	}
	return profile, nil
}
