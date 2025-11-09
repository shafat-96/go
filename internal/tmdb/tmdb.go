package tmdb

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"time"
)

type Movie struct {
	Title       string `json:"title"`
	ReleaseDate string `json:"release_date"`
}

type TV struct {
	Name string `json:"name"`
}

var httpClient = &http.Client{Timeout: 8 * time.Second}

const defaultAPIKey = "8baba8ab6b8bbe247645bcae7df63d0d"

func apiKey() string {
	if k := os.Getenv("TMDB_API_KEY"); k != "" {
		return k
	}
	return defaultAPIKey
}

func GetMovie(ctx context.Context, id string) (*Movie, error) {
	key := apiKey()
	url := fmt.Sprintf("https://api.themoviedb.org/3/movie/%s?api_key=%s", id, key)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := httpClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return nil, fmt.Errorf("tmdb: status %d", resp.StatusCode) }
	var m Movie
	if err := json.NewDecoder(resp.Body).Decode(&m); err != nil { return nil, err }
	return &m, nil
}

func GetTV(ctx context.Context, id string) (*TV, error) {
	key := apiKey()
	url := fmt.Sprintf("https://api.themoviedb.org/3/tv/%s?api_key=%s", id, key)
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	resp, err := httpClient.Do(req)
	if err != nil { return nil, err }
	defer resp.Body.Close()
	if resp.StatusCode != 200 { return nil, fmt.Errorf("tmdb: status %d", resp.StatusCode) }
	var t TV
	if err := json.NewDecoder(resp.Body).Decode(&t); err != nil { return nil, err }
	return &t, nil
}
