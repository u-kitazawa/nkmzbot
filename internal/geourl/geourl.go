package geourl

import (
	"errors"
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strconv"
	"time"
)

const (
	// RequestTimeout is the maximum time to wait for URL expansion
	RequestTimeout = 15 * time.Second
	// MaxRedirects is the maximum number of redirects to follow
	MaxRedirects = 10
)

var (
	reAt     = regexp.MustCompile(`@(-?\d+(?:\.\d+)?),(-?\d+(?:\.\d+)?)`)
	re3d4d   = regexp.MustCompile(`!3d(-?\d+(?:\.\d+)?)!4d(-?\d+(?:\.\d+)?)`)
	reQ      = regexp.MustCompile(`^\s*(-?\d+(?:\.\d+)?)\s*,\s*(-?\d+(?:\.\d+)?)\s*$`)
	reSearch = regexp.MustCompile(`/maps/search/(-?\d+(?:\.\d+)?),[ +]?(-?\d+(?:\.\d+)?)`)
)

// ExpandAndExtractCoords expands a Google Maps short URL and extracts coordinates from the final URL.
func ExpandAndExtractCoords(input string) (lat float64, lng float64, finalURL string, err error) {
	client := &http.Client{
		Timeout: RequestTimeout,
		// Follow redirects (default is fine); keep a safety cap.
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			if len(via) >= MaxRedirects {
				return errors.New("too many redirects")
			}
			return nil
		},
	}

	req, err := http.NewRequest("GET", input, nil)
	if err != nil {
		return 0, 0, "", err
	}
	// Some endpoints behave better with a UA.
	req.Header.Set("User-Agent", "Mozilla/5.0 (compatible; GeoTools/1.0)")
	req.Header.Set("Accept-Language", "ja,en;q=0.8")

	resp, err := client.Do(req)
	if err != nil {
		return 0, 0, "", err
	}
	defer resp.Body.Close()

	// After redirects, this is the final URL.
	if resp.Request == nil || resp.Request.URL == nil {
		return 0, 0, "", errors.New("failed to determine final URL")
	}
	finalURL = resp.Request.URL.String()

	lat, lng, ok := extractFromURL(finalURL)
	if !ok {
		return 0, 0, finalURL, fmt.Errorf("coordinates not found in final URL: %s", finalURL)
	}
	return lat, lng, finalURL, nil
}

func extractFromURL(s string) (lat, lng float64, ok bool) {
	// Pattern A: .../@lat,lng,zoom...
	if m := reAt.FindStringSubmatch(s); len(m) == 3 {
		return parse2(m[1], m[2])
	}
	// Pattern B: ...!3dlat!4dlng...
	if m := re3d4d.FindStringSubmatch(s); len(m) == 3 {
		return parse2(m[1], m[2])
	}

	// Pattern C: /maps/search/lat,+lng
	if m := reSearch.FindStringSubmatch(s); len(m) == 3 {
		return parse2(m[1], m[2])
	}

	// Pattern D: query params like ?q=lat,lng or ?query=lat,lng
	u, err := url.Parse(s)
	if err == nil {
		for _, key := range []string{"q", "query"} {
			if v := u.Query().Get(key); v != "" {
				if mm := reQ.FindStringSubmatch(v); len(mm) == 3 {
					return parse2(mm[1], mm[2])
				}
			}
		}
	}

	return 0, 0, false
}

func parse2(a, b string) (lat, lng float64, ok bool) {
	la, err1 := strconv.ParseFloat(a, 64)
	lo, err2 := strconv.ParseFloat(b, 64)
	if err1 != nil || err2 != nil {
		return 0, 0, false
	}
	return la, lo, true
}
