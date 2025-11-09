package flixhq

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"strconv"
	"strings"
	"time"

	"flixhq-api/internal/megacloud"
	"flixhq-api/internal/tmdb"

	"github.com/PuerkitoBio/goquery"
)

const (
	root      = "https://myflixerz.to"
	altRoot   = "https://flixhq.to"
	proxyBase = "https://test.1pimeshow.live"
)

var (
	httpClient = &http.Client{Timeout: 8 * time.Second}
	ua         = "Mozilla/5.0 (Windows NT 10.0; Win64; x64) AppleWebKit/537.36 (KHTML, like Gecko) Chrome/125 Safari/537.36"
	preferred  = map[string]bool{"megacloud": true, "upcloud": true, "akcloud": true}
	reEpisode  = regexp.MustCompile(`(\d+)`)
)

func normalizeTitle(s string) string { return strings.ToLower(strings.TrimSpace(s)) }

func convertToProxyURL(originalURL string) string {
	url := strings.TrimPrefix(originalURL, "https://")
	url = strings.TrimPrefix(url, "http://")
	return proxyBase + "/" + url
}

func reqHTML(ctx context.Context, url string) (string, error) {
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	req.Header.Set("User-Agent", ua)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	b, _ := io.ReadAll(resp.Body)
	return string(b), nil
}

func searchDetailURL(ctx context.Context, title, typ string, year int) (string, error) {
	clean := regexp.MustCompile(`[^a-zA-Z0-9\s-]`).ReplaceAllString(title, "")
	slug := strings.TrimSpace(regexp.MustCompile(`\s+`).ReplaceAllString(clean, "-"))
	url := fmt.Sprintf("%s/search/%s", root, slug)
	html, err := reqHTML(ctx, url)
	if err != nil {
		return "", err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", err
	}
	var found string
	doc.Find(".flw-item").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		itemType := normalizeTitle(s.Find(".float-right.fdi-type").Text())
		if itemType != typ {
			return true
		}
		name := normalizeTitle(s.Find("h2").Text())
		href, _ := s.Find("a").Attr("href")
		yr := strings.TrimSpace(s.Find(".fdi-item").First().Text())
		titleMatch := name == normalizeTitle(title)
		if typ == "movie" {
			if titleMatch && yr != "" {
				if y, _ := strconv.Atoi(yr); y == year {
					found = root + href
					return false
				}
			}
		} else {
			if titleMatch {
				found = root + href
				return false
			}
		}
		return true
	})
	return found, nil
}

func serverBroker(ctx context.Context, listURL string, preferredServer string) (string, string, error) {
	html, err := reqHTML(ctx, listURL)
	if err != nil {
		return "", "", err
	}
	doc, err := goquery.NewDocumentFromReader(strings.NewReader(html))
	if err != nil {
		return "", "", err
	}
	var sourceAPI, selectedServer string
	if preferredServer != "" {
		doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			serverName := normalizeTitle(s.Text())
			if serverName == preferredServer {
				if id, ok := s.Attr("data-id"); ok {
					sourceAPI = fmt.Sprintf("%s/ajax/episode/sources/%s", root, id)
					selectedServer = serverName
					return false
				}
			}
			return true
		})
	}
	if sourceAPI == "" {
		doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			serverName := normalizeTitle(s.Text())
			if serverName == "megacloud" {
				if id, ok := s.Attr("data-id"); ok {
					sourceAPI = fmt.Sprintf("%s/ajax/episode/sources/%s", root, id)
					selectedServer = "megacloud"
					return false
				}
			}
			return true
		})
	}
	if sourceAPI == "" {
		doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
			serverName := normalizeTitle(s.Text())
			if preferred[serverName] {
				if id, ok := s.Attr("data-id"); ok {
					sourceAPI = fmt.Sprintf("%s/ajax/episode/sources/%s", root, id)
					selectedServer = serverName
					return false
				}
			}
			return true
		})
	}
	if sourceAPI == "" {
		return "", "", nil
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, sourceAPI, nil)
	req.Header.Set("User-Agent", ua)
	resp, err := httpClient.Do(req)
	if err != nil {
		return "", "", err
	}
	defer resp.Body.Close()
	var obj map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&obj); err != nil {
		return "", "", err
	}
	if link, _ := obj["link"].(string); link != "" {
		return link, selectedServer, nil
	}
	return "", "", nil
}

func buildTvServerURL(ctx context.Context, detailURL string, seasonNum, episodeNum int) (string, error) {
	parts := strings.Split(detailURL, "-")
	id := parts[len(parts)-1]
	seasonsURL := fmt.Sprintf("%s/ajax/season/list/%s", root, id)
	html, err := reqHTML(ctx, seasonsURL)
	if err != nil {
		return "", err
	}
	doc, _ := goquery.NewDocumentFromReader(strings.NewReader(html))
	var seasonAjax string
	doc.Find("a.dropdown-item.ss-item").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		ssText := s.Text()
		ssParts := strings.FieldsFunc(ssText, func(r rune) bool { return r == ' ' || r == '_' })
		if len(ssParts) > 0 {
			last := ssParts[len(ssParts)-1]
			if n, _ := strconv.Atoi(last); n == seasonNum {
				if sid, ok := s.Attr("data-id"); ok {
					seasonAjax = fmt.Sprintf("%s/ajax/season/episodes/%s", root, sid)
					return false
				}
			}
		}
		return true
	})
	if seasonAjax == "" {
		return "", nil
	}
	html, err = reqHTML(ctx, seasonAjax)
	if err != nil {
		return "", err
	}
	doc, _ = goquery.NewDocumentFromReader(strings.NewReader(html))
	var serversURL string
	doc.Find("a").EachWithBreak(func(_ int, s *goquery.Selection) bool {
		epTxt := s.Find("strong").Text()
		match := reEpisode.FindStringSubmatch(epTxt)
		var ep string
		if len(match) == 2 {
			ep = match[1]
		}
		if ep != "" && strconv.Itoa(episodeNum) == ep {
			if id2, ok := s.Attr("data-id"); ok {
				serversURL = fmt.Sprintf("%s/ajax/episode/servers/%s", root, id2)
				return false
			}
		}
		return true
	})
	return serversURL, nil
}

func GetMovie(ctx context.Context, tmdbID string, preferredServer string) (map[string]any, error) {
	m, err := tmdb.GetMovie(ctx, tmdbID)
	if err != nil {
		return nil, err
	}
	year := 0
	if m.ReleaseDate != "" {
		if y, err := strconv.Atoi(strings.Split(m.ReleaseDate, "-")[0]); err == nil {
			year = y
		}
	}
	detailURL, err := searchDetailURL(ctx, m.Title, "movie", year)
	if err != nil || detailURL == "" {
		return nil, errors.New("FlixHQ: title not found")
	}
	parts := strings.Split(detailURL, "-")
	id := parts[len(parts)-1]
	listURL := fmt.Sprintf("%s/ajax/episode/list/%s", root, id)
	embed, _, err := serverBroker(ctx, listURL, preferredServer)
	if err != nil || embed == "" {
		return nil, errors.New("FlixHQ: no server link")
	}
	sources, tracks, err := megacloud.Extract(ctx, embed)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 || sources[0].File == "" {
		return nil, errors.New("FlixHQ: no sources")
	}
	proxyURL := convertToProxyURL(sources[0].File)
	res := map[string]any{
		"url":     proxyURL,
		"headers": map[string]string{"Referer": "https://megacloud.store/"},
	}
	if len(tracks) > 0 {
		var subs []map[string]string
		for _, t := range tracks {
			lang := t.Label
			if lang == "" {
				lang = t.Kind
				if lang == "" {
					lang = "sub"
				}
			}
			subs = append(subs, map[string]string{"url": t.File, "lang": lang})
		}
		res["subtitles"] = subs
	}
	return res, nil
}

func GetTv(ctx context.Context, tmdbID string, season, episode int, preferredServer string) (map[string]any, error) {
	t, err := tmdb.GetTV(ctx, tmdbID)
	if err != nil {
		return nil, err
	}
	detailURL, err := searchDetailURL(ctx, t.Name, "tv", 0)
	if err != nil || detailURL == "" {
		return nil, errors.New("FlixHQ: series not found")
	}
	serversURL, err := buildTvServerURL(ctx, detailURL, season, episode)
	if err != nil || serversURL == "" {
		return nil, errors.New("FlixHQ: episode not found")
	}
	embed, _, err := serverBroker(ctx, serversURL, preferredServer)
	if err != nil || embed == "" {
		return nil, errors.New("FlixHQ: no server link")
	}
	sources, tracks, err := megacloud.Extract(ctx, embed)
	if err != nil {
		return nil, err
	}
	if len(sources) == 0 || sources[0].File == "" {
		return nil, errors.New("FlixHQ: no sources")
	}
	proxyURL := convertToProxyURL(sources[0].File)
	res := map[string]any{
		"url":     proxyURL,
		"headers": map[string]string{"Referer": "https://megacloud.store/"},
	}
	if len(tracks) > 0 {
		var subs []map[string]string
		for _, t := range tracks {
			lang := t.Label
			if lang == "" {
				lang = t.Kind
				if lang == "" {
					lang = "sub"
				}
			}
			subs = append(subs, map[string]string{"url": t.File, "lang": lang})
		}
		res["subtitles"] = subs
	}
	return res, nil
}
