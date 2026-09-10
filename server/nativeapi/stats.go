package nativeapi

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/navidrome/navidrome/log"
	"github.com/navidrome/navidrome/model"
)

const (
	defaultStatsLimit = 20
	maxStatsLimit     = 100
)

// parseStatsRange resolves the [from, to] window for a recap/stats request.
// - "year" (e.g. ?year=2026) resolves to Jan 1 00:00:00 - Dec 31 23:59:59 UTC of that year.
// - "from"/"to" (RFC3339, or unix seconds) are used verbatim if given, instead of "year".
// - with neither, defaults to the current year.
func parseStatsRange(r *http.Request) (from, to time.Time, err error) {
	q := r.URL.Query()
	if f, t := q.Get("from"), q.Get("to"); f != "" || t != "" {
		from, err = parseStatsTime(f)
		if err != nil {
			return
		}
		to, err = parseStatsTime(t)
		return
	}

	year := time.Now().UTC().Year()
	if y := q.Get("year"); y != "" {
		year, err = strconv.Atoi(y)
		if err != nil {
			return
		}
	}
	from = time.Date(year, 1, 1, 0, 0, 0, 0, time.UTC)
	to = time.Date(year+1, 1, 1, 0, 0, 0, 0, time.UTC).Add(-time.Second)
	return
}

func parseStatsTime(s string) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if unix, err := strconv.ParseInt(s, 10, 64); err == nil {
		return time.Unix(unix, 0).UTC(), nil
	}
	return time.Parse(time.RFC3339, s)
}

func parseStatsLimit(r *http.Request) int {
	limit := defaultStatsLimit
	if l := r.URL.Query().Get("limit"); l != "" {
		if parsed, err := strconv.Atoi(l); err == nil && parsed > 0 {
			limit = parsed
		}
	}
	return min(limit, maxStatsLimit)
}

func writeStatsJSON(w http.ResponseWriter, r *http.Request, v any) {
	resp, err := json.Marshal(v)
	if err != nil {
		log.Error(r.Context(), "Error marshaling stats response", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}
	w.Header().Set("Content-Type", "application/json")
	_, _ = w.Write(resp) //nolint:gosec
}

func statsSummary(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		from, to, err := parseStatsRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		summary, err := ds.Scrobble(ctx).Summary(from, to)
		if err != nil {
			log.Error(ctx, "Error computing listen summary", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeStatsJSON(w, r, summary)
	}
}

func topSongs(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		from, to, err := parseStatsRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		songs, err := ds.Scrobble(ctx).TopSongs(from, to, parseStatsLimit(r))
		if err != nil {
			log.Error(ctx, "Error computing top songs", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeStatsJSON(w, r, songs)
	}
}

func topArtists(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		from, to, err := parseStatsRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		artists, err := ds.Scrobble(ctx).TopArtists(from, to, parseStatsLimit(r))
		if err != nil {
			log.Error(ctx, "Error computing top artists", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeStatsJSON(w, r, artists)
	}
}

func tasteProfile(ds model.DataStore) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		ctx := r.Context()
		from, to, err := parseStatsRange(r)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		profile, err := ds.Scrobble(ctx).TasteProfile(from, to)
		if err != nil {
			log.Error(ctx, "Error computing taste profile", err)
			http.Error(w, err.Error(), http.StatusInternalServerError)
			return
		}
		writeStatsJSON(w, r, profile)
	}
}
