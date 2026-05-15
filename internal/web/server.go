package web

import (
	"encoding/json"
	"io"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"rennes-emploi-dashboard/internal/dbstore"
	"rennes-emploi-dashboard/internal/filter"
	"rennes-emploi-dashboard/internal/francetravail"
	"rennes-emploi-dashboard/internal/mail"
	"rennes-emploi-dashboard/internal/poll"
)

type Server struct {
	Store     *dbstore.Store
	API       *francetravail.API
	PublicDir string
}

func envFloat(key string, def float64) float64 {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	v, err := strconv.ParseFloat(s, 64)
	if err != nil {
		return def
	}
	return v
}

func envIntMax0(key string, def int) int {
	s := strings.TrimSpace(os.Getenv(key))
	if s == "" {
		return def
	}
	v, err := strconv.Atoi(s)
	if err != nil {
		return def
	}
	if v < 0 {
		return 0
	}
	return v
}

func writeJSON(w http.ResponseWriter, status int, v interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

type eventDTO struct {
	Key           string   `json:"key"`
	Title         string   `json:"title"`
	StartAt       string   `json:"startAt"`
	LocationLabel string   `json:"locationLabel"`
	URL           string   `json:"url"`
	TypeLabel     string   `json:"typeLabel"`
	Lat           *float64 `json:"lat,omitempty"`
	Lon           *float64 `json:"lon,omitempty"`
	IsNew         bool     `json:"isNew"`
}

func (s *Server) publicDir() string {
	if d := strings.TrimSpace(s.PublicDir); d != "" {
		return d
	}
	wd, err := os.Getwd()
	if err != nil {
		return "public"
	}
	return filepath.Join(wd, "public")
}

func (s *Server) Handler() http.Handler {
	mux := http.NewServeMux()
	mux.HandleFunc("GET /api/health", s.getHealth)
	mux.HandleFunc("GET /api/status", s.getStatus)
	mux.HandleFunc("GET /api/config", s.getConfig)
	mux.HandleFunc("GET /api/events", s.getEvents)
	mux.HandleFunc("GET /api/subscribers", s.getSubscribers)
	mux.HandleFunc("POST /api/subscribers", s.postSubscribers)
	mux.HandleFunc("DELETE /api/subscribers/{email}", s.deleteSubscriber)
	mux.HandleFunc("POST /api/test-mail", s.postTestMail)
	mux.HandleFunc("POST /api/poll-once", s.postPollOnce)
	mux.Handle("GET /", http.FileServer(http.Dir(s.publicDir())))
	return mux
}

func (s *Server) getHealth(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ok": true,
		"ts": time.Now().UTC().Format(time.RFC3339Nano),
	})
}

func (s *Server) getStatus(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"ts":             time.Now().UTC().Format(time.RFC3339Nano),
		"mockMode":       francetravail.IsMockMode(),
		"franceTravail":  s.API.Probe(),
		"smtpConfigured": mail.IsSmtpConfigured(),
		"lastPoll":       poll.GetLastPoll(),
	})
}

func (s *Server) getConfig(w http.ResponseWriter, r *http.Request) {
	n := 0
	if c, err := s.Store.SubscriberCount(); err == nil {
		n = c
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"center": map[string]float64{
			"lat": envFloat("RENNES_LAT", 48.1173),
			"lon": envFloat("RENNES_LON", -1.6778),
		},
		"radiusKm":         envFloat("DEFAULT_RADIUS_KM", 40),
		"mockMode":         francetravail.IsMockMode(),
		"subscriberCount":  n,
		"uiPollIntervalMs": envIntMax0("UI_POLL_INTERVAL_MS", 0),
	})
}

func (s *Server) getEvents(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	eq := francetravail.EventQuery{
		Lat:       strings.TrimSpace(os.Getenv("RENNES_LAT")),
		Lon:       strings.TrimSpace(os.Getenv("RENNES_LON")),
		RadiusKm:  strings.TrimSpace(os.Getenv("DEFAULT_RADIUS_KM")),
		DateFrom:  q.Get("from"),
		DateTo:    q.Get("to"),
		Q:         q.Get("q"),
		Type:      q.Get("type"),
	}
	source, raw, err := s.API.GetEventsForDashboard(eq)
	if err != nil {
		writeJSON(w, http.StatusBadGateway, map[string]interface{}{
			"error": err.Error(),
			"hint":  "Vérifie FT_CLIENT_ID/SECRET, FT_OAUTH_SCOPE et FT_EVENTS_PATH sur francetravail.io — ou active MOCK_FT=true.",
		})
		return
	}
	fq := filter.Query{
		DateFrom: eq.DateFrom,
		DateTo:   eq.DateTo,
		Type:     eq.Type,
		Q:        eq.Q,
	}
	events := filter.FilterEvents(raw, fq)
	out := make([]eventDTO, 0, len(events))
	for _, ev := range events {
		out = append(out, eventDTO{
			Key: ev.Key, Title: ev.Title, StartAt: ev.StartAt,
			LocationLabel: ev.LocationLabel, URL: ev.URL, TypeLabel: ev.TypeLabel,
			Lat: ev.Lat, Lon: ev.Lon,
			IsNew: poll.IsNewBadge(s.Store, ev.Key),
		})
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"source": source,
		"count":  len(out),
		"events": out,
	})
}

func (s *Server) getSubscribers(w http.ResponseWriter, r *http.Request) {
	list, err := s.Store.ListSubscribers()
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	if list == nil {
		list = []dbstore.Subscriber{}
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"subscribers": list})
}

func (s *Server) postSubscribers(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&body); err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
		return
	}
	email, err := s.Store.AddSubscriber(body.Email)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{"ok": true, "email": email})
}

func (s *Server) deleteSubscriber(w http.ResponseWriter, r *http.Request) {
	raw := r.PathValue("email")
	em, err := url.PathUnescape(raw)
	if err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email invalide"})
		return
	}
	if err := s.Store.RemoveSubscriber(em); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) postTestMail(w http.ResponseWriter, r *http.Request) {
	var body struct {
		Email string `json:"email"`
	}
	if err := json.NewDecoder(io.LimitReader(r.Body, 1<<14)).Decode(&body); err != nil && err != io.EOF {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "JSON invalide"})
		return
	}
	to := strings.TrimSpace(body.Email)
	if to == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "email requis"})
		return
	}
	if err := mail.SendTest(to); err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, map[string]bool{"ok": true})
}

func (s *Server) postPollOnce(w http.ResponseWriter, r *http.Request) {
	out, err := poll.PollOnce(s.API, s.Store)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, out)
}
