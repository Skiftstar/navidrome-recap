package subsonic

import (
	"net/http"

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
// tracks similar to id by Euclidean distance across the VibeNet audio-feature
// tags, computed entirely from Navidrome's own data (see
// model.MediaFileRepository.SimilarByVibe).
func (api *Router) GetVibeSimilarTracks(r *http.Request) (*responses.Subsonic, error) {
	ctx := r.Context()
	p := req.Params(r)
	id, err := p.String("id")
	if err != nil {
		return nil, newError(responses.ErrorMissingParameter, "missing parameter: 'id'")
	}
	count := p.IntOr("count", defaultVibeSimilarLimit)
	if count <= 0 {
		count = defaultVibeSimilarLimit
	}
	count = min(count, maxVibeSimilarLimit)

	results, err := api.ds.MediaFile(ctx).SimilarByVibe(id, count)
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
