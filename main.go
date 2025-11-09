package main

import (
	"encoding/json"
	"log"
	"net/http"
	"os"
	"strconv"
	"time"

	"flixhq-api/internal/flixhq"
)

func main() {
	addr := getEnv("HOST", "localhost") + ":" + getEnv("PORT", "3100")
	mux := http.NewServeMux()
	mux.HandleFunc("/movie/", corsMiddleware(movieHandler))
	mux.HandleFunc("/tv/", corsMiddleware(tvHandler))
	server := &http.Server{
		Addr:              addr,
		Handler:           mux,
		ReadHeaderTimeout: 10 * time.Second,
	}
	log.Printf("FlixHQ API listening on http://%s", addr)
	log.Fatal(server.ListenAndServe())
}

func getEnv(k, d string) string {
	if v := os.Getenv(k); v != "" { return v }
	return d
}

func writeJSON(w http.ResponseWriter, code int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(v)
}

func movieHandler(w http.ResponseWriter, r *http.Request) {
	// /movie/{tmdbID}
	parts := splitPath(r.URL.Path)
	if len(parts) != 2 || parts[0] != "movie" {
		writeJSON(w, 400, map[string]string{"error": "invalid path"})
		return
	}
	tmdbID := parts[1]
	server := r.URL.Query().Get("server")
	ctx := r.Context()
	res, err := flixhq.GetMovie(ctx, tmdbID, server)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, res)
}

func tvHandler(w http.ResponseWriter, r *http.Request) {
	// /tv/{tmdbID}/{season}/{episode}
	parts := splitPath(r.URL.Path)
	if len(parts) != 4 || parts[0] != "tv" {
		writeJSON(w, 400, map[string]string{"error": "invalid path"})
		return
	}
	tmdbID := parts[1]
	season, err1 := strconv.Atoi(parts[2])
	episode, err2 := strconv.Atoi(parts[3])
	if err1 != nil || err2 != nil {
		writeJSON(w, 400, map[string]string{"error": "invalid season/episode"})
		return
	}
	server := r.URL.Query().Get("server")
	ctx := r.Context()
	res, err := flixhq.GetTv(ctx, tmdbID, season, episode, server)
	if err != nil {
		writeJSON(w, 404, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, 200, res)
}

func splitPath(p string) []string {
	if len(p) > 0 && p[0] == '/' { p = p[1:] }
	if p == "" { return nil }
	// simple split
	var parts []string
	start := 0
	for i := 0; i < len(p); i++ {
		if p[i] == '/' {
			if start < i { parts = append(parts, p[start:i]) }
			start = i+1
		}
	}
	if start <= len(p)-1 { parts = append(parts, p[start:]) }
	return parts
}

func corsMiddleware(next http.HandlerFunc) http.HandlerFunc {
    return func(w http.ResponseWriter, r *http.Request) {
        w.Header().Set("Access-Control-Allow-Origin", "*")
        w.Header().Set("Access-Control-Allow-Methods", "GET, OPTIONS")
        w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, Range")
        if r.Method == http.MethodOptions {
            w.WriteHeader(http.StatusOK)
            return
        }
        next(w, r)
    }
}
