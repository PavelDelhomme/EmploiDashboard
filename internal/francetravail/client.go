package francetravail

import (
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"math"
	"net/http"
	"net/url"
	"os"
	"strconv"
	"strings"
	"sync"
	"time"

	"rennes-emploi-dashboard/internal/filter"
	"rennes-emploi-dashboard/internal/scraperclient"
)

type API struct {
	mu          sync.Mutex
	accessToken string
	tokenExp    time.Time
	HTTP        *http.Client
	Scraper     *scraperclient.Client
}

func New() *API {
	return &API{HTTP: &http.Client{Timeout: 45 * time.Second}}
}

func boolEnv(name string, def bool) bool {
	v := strings.TrimSpace(os.Getenv(name))
	if v == "" {
		return def
	}
	v = strings.ToLower(v)
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

func IsMockMode() bool { return boolEnv("MOCK_FT", false) }

func portalBase() string {
	b := strings.TrimSpace(os.Getenv("FT_PORTAL_BASE_URL"))
	if b == "" {
		b = "https://mesevenementsemploi.francetravail.fr/mes-evenements-emploi"
	}
	return strings.TrimRight(b, "/")
}

func (a *API) getAccessToken() (string, error) {
	a.mu.Lock()
	if a.accessToken != "" && time.Now().Before(a.tokenExp.Add(-30*time.Second)) {
		t := a.accessToken
		a.mu.Unlock()
		return t, nil
	}
	a.mu.Unlock()

	cid := strings.TrimSpace(os.Getenv("FT_CLIENT_ID"))
	sec := strings.TrimSpace(os.Getenv("FT_CLIENT_SECRET"))
	if cid == "" || sec == "" {
		return "", fmt.Errorf("FT_CLIENT_ID / FT_CLIENT_SECRET manquants")
	}
	scope := os.Getenv("FT_OAUTH_SCOPE")
	if scope == "" {
		scope = "api_evenementsv1 evenements"
	}
	tokenURL := os.Getenv("FT_TOKEN_URL")
	if tokenURL == "" {
		tokenURL = "https://entreprise.francetravail.fr/connexion/oauth2/access_token?realm=/partenaire"
	}
	form := url.Values{}
	form.Set("grant_type", "client_credentials")
	form.Set("client_id", cid)
	form.Set("client_secret", sec)
	form.Set("scope", scope)
	req, err := http.NewRequest(http.MethodPost, tokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := a.HTTP.Do(req)
	if err != nil {
		return "", err
	}
	defer res.Body.Close()
	body, _ := io.ReadAll(res.Body)
	if res.StatusCode != 200 {
		return "", fmt.Errorf("token FT HTTP %d — %s", res.StatusCode, truncate(string(body), 500))
	}
	var tok struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tok); err != nil {
		return "", fmt.Errorf("token non JSON: %w", err)
	}
	if tok.AccessToken == "" {
		return "", fmt.Errorf("pas d’access_token")
	}
	ttl := tok.ExpiresIn
	if ttl < 60 {
		ttl = 1200
	}
	exp := time.Now().Add(time.Duration(ttl) * time.Second)

	a.mu.Lock()
	defer a.mu.Unlock()
	if a.accessToken != "" && time.Now().Before(a.tokenExp.Add(-30*time.Second)) {
		return a.accessToken, nil
	}
	a.accessToken = tok.AccessToken
	a.tokenExp = exp
	return a.accessToken, nil
}

func (a *API) Probe() map[string]interface{} {
	if IsMockMode() {
		return map[string]interface{}{"mode": "mock", "credentialsSet": true, "tokenOk": true}
	}
	if strings.TrimSpace(os.Getenv("FT_CLIENT_ID")) == "" || strings.TrimSpace(os.Getenv("FT_CLIENT_SECRET")) == "" {
		return map[string]interface{}{"mode": "live", "credentialsSet": false, "tokenOk": false}
	}
	if _, err := a.getAccessToken(); err != nil {
		return map[string]interface{}{"mode": "live", "credentialsSet": true, "tokenOk": false, "error": err.Error()}
	}
	return map[string]interface{}{"mode": "live", "credentialsSet": true, "tokenOk": true}
}

func joinEventsURL() (*url.URL, error) {
	base := os.Getenv("FT_API_BASE")
	if base == "" {
		base = "https://api.francetravail.io/"
	}
	path := os.Getenv("FT_EVENTS_PATH")
	if path == "" {
		path = "partenaire/evenements/v1/evenements"
	}
	path = strings.TrimLeft(path, "/")
	u, err := url.Parse(base)
	if err != nil {
		return nil, err
	}
	if !strings.HasSuffix(u.Path, "/") && u.Path != "" {
		u.Path += "/"
	}
	ref, err := url.Parse(path)
	if err != nil {
		return nil, err
	}
	return u.ResolveReference(ref), nil
}

type EventQuery struct {
	Lat, Lon, RadiusKm, DateFrom, DateTo, Q, Type string
}

func buildQuery(eq EventQuery) url.Values {
	sp := url.Values{}
	lat := eq.Lat
	if lat == "" {
		lat = os.Getenv("RENNES_LAT")
	}
	lon := eq.Lon
	if lon == "" {
		lon = os.Getenv("RENNES_LON")
	}
	dist := eq.RadiusKm
	if dist == "" {
		dist = os.Getenv("DEFAULT_RADIUS_KM")
	}
	if dist == "" {
		dist = "40"
	}
	if lat != "" {
		sp.Set("latitude", lat)
	}
	if lon != "" {
		sp.Set("longitude", lon)
	}
	if dist != "" {
		sp.Set("distance", dist)
	}
	if cp := strings.TrimSpace(os.Getenv("FT_FILTER_CODE_POSTAL")); cp != "" {
		sp.Set("codePostal", cp)
	}
	if dep := strings.TrimSpace(os.Getenv("FT_FILTER_DEPARTEMENT")); dep != "" {
		sp.Set("codeDepartement", dep)
	}
	if eq.DateFrom != "" {
		sp.Set("dateDebut", eq.DateFrom)
	}
	if eq.DateTo != "" {
		sp.Set("dateFin", eq.DateTo)
	}
	if eq.Q != "" {
		sp.Set("motsCles", eq.Q)
	}
	if eq.Type != "" {
		sp.Set("type", eq.Type)
	}
	if extra := strings.TrimSpace(os.Getenv("FT_EXTRA_QUERY")); extra != "" {
		if strings.HasPrefix(extra, "?") {
			extra = extra[1:]
		}
		if q2, err := url.ParseQuery(extra); err == nil {
			for k, vs := range q2 {
				for _, v := range vs {
					sp.Add(k, v)
				}
			}
		}
	}
	return sp
}

func parseContentRange(h string) (start, end, total int, ok bool) {
	for _, part := range strings.Split(h, " ") {
		if strings.Contains(part, "/") && strings.Contains(part, "-") {
			var a, b, t int
			if n, _ := fmt.Sscanf(part, "%d-%d/%d", &a, &b, &t); n == 3 {
				return a, b, t, true
			}
		}
	}
	return 0, 0, 0, false
}

func extractEventsArray(root interface{}) []interface{} {
	if arr, ok := root.([]interface{}); ok {
		return arr
	}
	m, ok := root.(map[string]interface{})
	if !ok {
		return nil
	}
	keys := []string{"resultats", "evenements", "evenementsEmploi", "listeEvenements", "events", "content", "records", "items"}
	for _, k := range keys {
		if v, ok := m[k].([]interface{}); ok {
			return v
		}
	}
	if emb, ok := m["_embedded"].(map[string]interface{}); ok {
		for _, k := range keys {
			if v, ok := emb[k].([]interface{}); ok {
				return v
			}
		}
	}
	return nil
}

func pickString(m map[string]interface{}, keys ...string) string {
	for _, k := range keys {
		if v, ok := m[k]; ok && v != nil {
			s := strings.TrimSpace(fmt.Sprint(v))
			if s != "" {
				return s
			}
		}
	}
	return ""
}

func normalizeLieu(raw map[string]interface{}) map[string]interface{} {
	l := raw["lieu"]
	if s, ok := l.(string); ok {
		return map[string]interface{}{"libelle": s}
	}
	if m, ok := l.(map[string]interface{}); ok {
		return m
	}
	return map[string]interface{}{}
}

func toISO(v string) string {
	if v == "" {
		return ""
	}
	for _, layout := range []string{time.RFC3339, time.RFC3339Nano, "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC().Format(time.RFC3339Nano)
		}
	}
	return v
}

func resolvePublicURL(raw map[string]interface{}, key, title, startAt, loc string) string {
	direct := pickString(raw, "url", "lien", "urlInscription", "urlDetail", "lienDetail", "lienWeb")
	if direct != "" {
		return direct
	}
	tpl := strings.TrimSpace(os.Getenv("FT_PORTAL_EVENT_URL_TEMPLATE"))
	portal := portalBase()
	if tpl != "" {
		id := pickString(raw, "id", "idEvenement", "identifiant", "identifiantEvenement", "numero", "codeEvenement")
		if id == "" {
			id = key
		}
		out := strings.ReplaceAll(tpl, "{id}", url.QueryEscape(id))
		out = strings.ReplaceAll(out, "{portal}", portal)
		return out
	}
	return portal + "/evenements"
}

func normalizeEvent(raw map[string]interface{}) filter.Event {
	id := pickString(raw, "id", "idEvenement", "identifiant", "identifiantEvenement", "numero", "codeEvenement")
	title := pickString(raw, "titre", "intitule", "libelle", "nom", "title")
	startAt := toISO(pickString(raw, "dateDebut", "dateHeureDebut", "dateDebutEvenement", "date", "horaireDebut", "startDate"))
	lieu := normalizeLieu(raw)
	parts := []string{
		pickString(raw, "ville", "commune", "libelleCommune"),
		pickString(lieu, "libelle", "nom", "ville", "commune", "intitule"),
		pickString(raw, "codePostal", "code_postal"),
		pickString(lieu, "codePostal", "code_postal"),
	}
	seen := map[string]bool{}
	var loc []string
	for _, p := range parts {
		p = strings.TrimSpace(p)
		if p == "" || seen[p] {
			continue
		}
		seen[p] = true
		loc = append(loc, p)
	}
	locationLabel := strings.Join(loc, " ")
	latStr := pickString(raw, "latitude")
	if latStr == "" {
		latStr = pickString(lieu, "latitude", "lat", "coordLat")
	}
	lonStr := pickString(raw, "longitude")
	if lonStr == "" {
		lonStr = pickString(lieu, "longitude", "lon", "coordLon")
	}
	typeLabel := pickString(raw, "type", "typeEvenement", "libelleType", "categorie", "nature")
	k := id
	if k == "" {
		k = fmt.Sprintf("%s|%s|%s", title, startAt, locationLabel)
	}
	if len(k) > 512 {
		k = k[:512]
	}
	if title == "" {
		title = "Événement emploi"
	}
	ev := filter.Event{
		Key: k, Title: title, StartAt: startAt, LocationLabel: locationLabel,
		URL: resolvePublicURL(raw, k, title, startAt, locationLabel), TypeLabel: typeLabel,
	}
	if la, err := strconv.ParseFloat(latStr, 64); err == nil && !math.IsNaN(la) {
		ev.Lat = &la
	}
	if lo, err := strconv.ParseFloat(lonStr, 64); err == nil && !math.IsNaN(lo) {
		ev.Lon = &lo
	}
	return ev
}

func dedupe(events []filter.Event) []filter.Event {
	seen := map[string]struct{}{}
	var out []filter.Event
	for _, e := range events {
		if _, ok := seen[e.Key]; ok {
			continue
		}
		seen[e.Key] = struct{}{}
		out = append(out, e)
	}
	return out
}

var errRangeNotSupported = errors.New("ft range 416")

func (a *API) FetchEvents(eq EventQuery) ([]filter.Event, error) {
	tok, err := a.getAccessToken()
	if err != nil {
		return nil, err
	}
	u, err := joinEventsURL()
	if err != nil {
		return nil, err
	}
	u.RawQuery = buildQuery(eq).Encode()
	pageSize := 200
	if v := strings.TrimSpace(os.Getenv("FT_RANGE_PAGE_SIZE")); v != "" {
		if n, e := strconv.Atoi(v); e == nil {
			pageSize = max(10, min(500, n))
		}
	}
	useRange := boolEnv("FT_USE_RANGE_HEADER", false)

	fetchOnce := func(rangeHdr string) (*http.Response, map[string]interface{}, error) {
		req, err := http.NewRequest(http.MethodGet, u.String(), nil)
		if err != nil {
			return nil, nil, err
		}
		req.Header.Set("Authorization", "Bearer "+tok)
		req.Header.Set("Accept", "application/json")
		if rangeHdr != "" {
			req.Header.Set("Range", rangeHdr)
		}
		res, err := a.HTTP.Do(req)
		if err != nil {
			return nil, nil, err
		}
		body, _ := io.ReadAll(res.Body)
		res.Body.Close()
		if res.StatusCode == 416 && rangeHdr != "" {
			return res, nil, errRangeNotSupported
		}
		if res.StatusCode != 200 {
			return nil, nil, fmt.Errorf("API événements FT HTTP %d — %s", res.StatusCode, truncate(string(body), 400))
		}
		var root map[string]interface{}
		if err := json.Unmarshal(body, &root); err != nil {
			return nil, nil, fmt.Errorf("réponse non JSON: %w", err)
		}
		return res, root, nil
	}

	if !useRange {
		_, root, err := fetchOnce("")
		if err != nil {
			return nil, err
		}
		return eventsFromRoot(root), nil
	}

	var all []filter.Event
	start := 0
	for guard := 0; guard < 60; guard++ {
		end := start + pageSize - 1
		res, root, err := fetchOnce(fmt.Sprintf("items=%d-%d", start, end))
		if err == errRangeNotSupported {
			_, root2, err2 := fetchOnce("")
			if err2 != nil {
				return nil, err2
			}
			return eventsFromRoot(root2), nil
		}
		if err != nil {
			return nil, err
		}
		arr := extractEventsArray(root)
		for _, it := range arr {
			if m, ok := it.(map[string]interface{}); ok {
				all = append(all, normalizeEvent(m))
			}
		}
		cr := res.Header.Get("Content-Range")
		if cr == "" {
			cr = res.Header.Get("content-range")
		}
		_, b, tot, ok := parseContentRange(cr)
		if !ok || len(arr) == 0 {
			break
		}
		if b+1 >= tot {
			break
		}
		start = b + 1
	}
	return dedupe(all), nil
}

func eventsFromRoot(root map[string]interface{}) []filter.Event {
	var out []filter.Event
	for _, it := range extractEventsArray(root) {
		if m, ok := it.(map[string]interface{}); ok {
			out = append(out, normalizeEvent(m))
		}
	}
	return dedupe(out)
}

func truncate(s string, n int) string {
	if len(s) <= n {
		return s
	}
	return s[:n]
}

func MockEvents() []filter.Event {
	base := time.Now().UTC()
	d := func(days int) string {
		return base.AddDate(0, 0, days).Format(time.RFC3339Nano)
	}
	p := portalBase() + "/evenements"
	return []filter.Event{
		{Key: "mock-forum-rennes-1", Title: "Forum de l’emploi — Rennes (démo)", StartAt: d(2), LocationLabel: "Rennes 35000", Lat: f64(48.1173), Lon: f64(-1.6778), URL: p, TypeLabel: "Forum"},
		{Key: "mock-jobdating-cesson-1", Title: "Job dating — Cesson-Sévigné (démo)", StartAt: d(4), LocationLabel: "Cesson-Sévigné 35510", Lat: f64(48.1192), Lon: f64(-1.6036), URL: p, TypeLabel: "Job dating"},
		{Key: "mock-atelier-saintmalo-1", Title: "Atelier CV — Saint-Malo (démo, hors fenêtre dates)", StartAt: d(55), LocationLabel: "Saint-Malo 35400", Lat: f64(48.6493), Lon: f64(-2.0075), URL: p, TypeLabel: "Atelier"},
	}
}

func mergeEventsPreferAPI(apiEvs, scraped []filter.Event) []filter.Event {
	seen := make(map[string]struct{}, len(apiEvs)+len(scraped))
	out := make([]filter.Event, 0, len(apiEvs)+len(scraped))
	for _, e := range apiEvs {
		if e.Key == "" {
			continue
		}
		if _, ok := seen[e.Key]; ok {
			continue
		}
		seen[e.Key] = struct{}{}
		out = append(out, e)
	}
	for _, e := range scraped {
		if e.Key == "" {
			continue
		}
		if _, ok := seen[e.Key]; ok {
			continue
		}
		seen[e.Key] = struct{}{}
		out = append(out, e)
	}
	return out
}

func f64(v float64) *float64 { return &v }

func (a *API) GetEventsForDashboard(eq EventQuery) (source string, events []filter.Event, err error) {
	if IsMockMode() {
		return "mock", MockEvents(), nil
	}
	evs, err := a.FetchEvents(eq)
	if err != nil {
		return "", nil, err
	}
	source = "francetravail"
	out := evs
	if a.Scraper != nil {
		se, err2 := a.Scraper.Fetch(scraperclient.Params{
			Lat: eq.Lat, Lon: eq.Lon, RadiusKm: eq.RadiusKm,
			DateFrom: eq.DateFrom, DateTo: eq.DateTo,
		})
		if err2 != nil {
			log.Printf("camoufox scraper: %v", err2)
		} else if len(se) > 0 {
			out = mergeEventsPreferAPI(evs, se)
			source = "francetravail+camoufox"
		}
	}
	return source, out, nil
}
