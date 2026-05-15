package scraperclient

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"rennes-emploi-dashboard/internal/filter"
)

// Params aligne les filtres géo/dates envoyés au service Camoufox (HTTP).
type Params struct {
	Lat, Lon, RadiusKm, DateFrom, DateTo string
}

type Client struct {
	base   string
	http   *http.Client
	ttl    time.Duration
	mu     sync.Mutex
	cacheK string
	cacheT time.Time
	cacheE []filter.Event
}

// FromEnv crée un client si CAMOUFOX_SCRAPER_URL est défini (ex. http://camoufox-scraper:8765).
func FromEnv() *Client {
	base := strings.TrimSpace(os.Getenv("CAMOUFOX_SCRAPER_URL"))
	if base == "" {
		return nil
	}
	base = strings.TrimRight(base, "/")
	ttl := 5 * time.Minute
	if s := strings.TrimSpace(os.Getenv("SCRAPER_CACHE_TTL_SEC")); s != "" {
		if n, err := strconv.Atoi(s); err == nil && n > 0 {
			ttl = time.Duration(n) * time.Second
		}
	}
	return &Client{
		base: base,
		http: &http.Client{Timeout: 120 * time.Second},
		ttl:  ttl,
	}
}

func (c *Client) cacheKey(p Params) string {
	return strings.Join([]string{p.Lat, p.Lon, p.RadiusKm, p.DateFrom, p.DateTo}, "\x1e")
}

func (c *Client) getCached(key string) ([]filter.Event, bool) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cacheK != key || time.Since(c.cacheT) > c.ttl {
		return nil, false
	}
	return c.cacheE, true
}

func (c *Client) setCached(key string, evs []filter.Event) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.cacheK = key
	c.cacheT = time.Now()
	c.cacheE = evs
}

// Fetch interroge le service Camoufox (GET /events). Erreur réseau / HTTP → erreur.
func (c *Client) Fetch(p Params) ([]filter.Event, error) {
	if c == nil || c.base == "" {
		return nil, nil
	}
	key := c.cacheKey(p)
	if evs, ok := c.getCached(key); ok {
		out := make([]filter.Event, len(evs))
		copy(out, evs)
		return out, nil
	}
	q := url.Values{}
	if p.Lat != "" {
		q.Set("lat", p.Lat)
	}
	if p.Lon != "" {
		q.Set("lon", p.Lon)
	}
	if p.RadiusKm != "" {
		q.Set("radius_km", p.RadiusKm)
	}
	if p.DateFrom != "" {
		q.Set("from", p.DateFrom)
	}
	if p.DateTo != "" {
		q.Set("to", p.DateTo)
	}
	u := c.base + "/events?" + q.Encode()
	res, err := c.http.Get(u)
	if err != nil {
		return nil, err
	}
	defer res.Body.Close()
	body, err := io.ReadAll(io.LimitReader(res.Body, 8<<20))
	if err != nil {
		return nil, err
	}
	if res.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("scraper HTTP %d: %s", res.StatusCode, truncate(string(body), 300))
	}
	var parsed struct {
		Events []filter.Event `json:"events"`
	}
	if err := json.Unmarshal(body, &parsed); err != nil {
		return nil, fmt.Errorf("scraper JSON: %w", err)
	}
	c.setCached(key, parsed.Events)
	return parsed.Events, nil
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}
