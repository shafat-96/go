package megacloud

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

const (
	mainURL   = "https://videostr.net"
	userAgent = "Mozilla/5.0 (Linux; Android 10; K) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/137.0.0.0 Mobile Safari/537.36"
)

var (
	httpClient = &http.Client{Timeout: 8 * time.Second}
	reID       = regexp.MustCompile(`data-id="([^"]+)"`)
	reNonce48  = regexp.MustCompile(`\b[a-zA-Z0-9]{48}\b`)
	reNonce16x3 = regexp.MustCompile(`\b([a-zA-Z0-9]{16})\b.*?\b([a-zA-Z0-9]{16})\b.*?\b([a-zA-Z0-9]{16})\b`)
)

type Source struct { File string `json:"file"` ; Type string `json:"type"` }

type Track struct { File string `json:"file"` ; Label string `json:"label"` ; Kind string `json:"kind"` }

type resultPayload struct {
	Sources any     `json:"sources"`
	Tracks  []Track `json:"tracks"`
	T       int     `json:"t"`
	Server  int     `json:"server"`
}

// Extract fetches the embed page and then calls videostr getSources
func Extract(ctx context.Context, embed string) (sources []Source, tracks []Track, err error) {
	// GET embed page
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, embed, nil)
	req.Header.Set("User-Agent", userAgent)
	req.Header.Set("Referer", embed)
	resp, err := httpClient.Do(req)
	if err != nil { return nil, nil, err }
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	html := string(b)

	// file id
	id := ""
	if m := reID.FindStringSubmatch(html); len(m) == 2 { id = m[1] }
	if id == "" { return nil, nil, errors.New("megacloud: file id not found") }
	// nonce
	nonce := ""
	if m := reNonce48.FindString(html); m != "" {
		nonce = m
	} else if m := reNonce16x3.FindStringSubmatch(html); len(m) == 4 {
		nonce = m[1] + m[2] + m[3]
	}
	if nonce == "" { return nil, nil, errors.New("megacloud: nonce not found") }

	u, _ := url.Parse(embed)
	api := fmt.Sprintf("%s/embed-1/v3/e-1/getSources?id=%s&_k=%s", mainURL, id, nonce)
	preq, _ := http.NewRequestWithContext(ctx, http.MethodGet, api, nil)
	preq.Header.Set("Accept", "*/*")
	preq.Header.Set("X-Requested-With", "XMLHttpRequest")
	preq.Header.Set("Referer", u.String())
	preq.Header.Set("User-Agent", userAgent)
	presp, err := httpClient.Do(preq)
	if err != nil { return nil, nil, err }
	defer presp.Body.Close()
	pb, _ := io.ReadAll(presp.Body)
	var payload resultPayload
	if err := json.Unmarshal(pb, &payload); err != nil { return nil, nil, err }

	// normalize sources
	switch v := payload.Sources.(type) {
	case []any:
		for _, it := range v {
			if m, ok := it.(map[string]any); ok {
				file, _ := m["file"].(string)
				typ, _ := m["type"].(string)
				if typ == "" { typ = "hls" }
				sources = append(sources, Source{File: file, Type: typ})
			}
		}
	case string:
		sources = append(sources, Source{File: v, Type: "hls"})
	}
	// filter tracks
	for _, t := range payload.Tracks {
		k := strings.ToLower(t.Kind)
		if k == "captions" || k == "subtitles" {
			tracks = append(tracks, t)
		}
	}
	return sources, tracks, nil
}
