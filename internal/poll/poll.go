package poll

import (
	"log"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/robfig/cron/v3"

	"rennes-emploi-dashboard/internal/dbstore"
	"rennes-emploi-dashboard/internal/filter"
	"rennes-emploi-dashboard/internal/francetravail"
	"rennes-emploi-dashboard/internal/mail"
)

const newBadgeHours = 36
const seedStamp = "2000-01-01T00:00:00.000Z"

func boolEnv(name string, def bool) bool {
	v := strings.TrimSpace(strings.ToLower(os.Getenv(name)))
	if v == "" {
		return def
	}
	return v == "1" || v == "true" || v == "yes" || v == "on"
}

// LastPollInfo reflète la forme attendue par le frontend (lastPoll).
type LastPollInfo struct {
	At      *string     `json:"at"`
	Ok      *bool       `json:"ok"`
	Summary interface{} `json:"summary"`
}

var (
	lastMu   sync.Mutex
	lastPoll = LastPollInfo{At: nil, Ok: nil, Summary: nil}
)

func GetLastPoll() LastPollInfo {
	lastMu.Lock()
	defer lastMu.Unlock()
	return lastPoll
}

func setLastOk(at string, summary interface{}) {
	lastMu.Lock()
	defer lastMu.Unlock()
	ok := true
	lastPoll = LastPollInfo{At: &at, Ok: &ok, Summary: summary}
}

func setLastErr(at string, err error) {
	lastMu.Lock()
	defer lastMu.Unlock()
	ok := false
	lastPoll = LastPollInfo{
		At:      &at,
		Ok:      &ok,
		Summary: map[string]string{"error": err.Error()},
	}
}

// IsNewBadge : badge « NOUVEAU » si l’événement a été vu récemment en base (même logique que Node).
func IsNewBadge(store *dbstore.Store, eventKey string) bool {
	fs, err := store.FirstSeenAt(eventKey)
	if err != nil || fs == "" {
		return false
	}
	t, err := time.Parse(time.RFC3339Nano, fs)
	if err != nil {
		t, err = time.Parse(time.RFC3339, fs)
	}
	if err != nil {
		return false
	}
	return time.Since(t) < newBadgeHours*time.Hour
}

func dashboardQuery() francetravail.EventQuery {
	return francetravail.EventQuery{
		Lat:       strings.TrimSpace(os.Getenv("RENNES_LAT")),
		Lon:       strings.TrimSpace(os.Getenv("RENNES_LON")),
		RadiusKm:  strings.TrimSpace(os.Getenv("DEFAULT_RADIUS_KM")),
		DateFrom:  "",
		DateTo:    "",
		Q:         "",
		Type:      "",
	}
}

func pollOnceInternal(api *francetravail.API, store *dbstore.Store) (map[string]interface{}, error) {
	source, events, err := api.GetEventsForDashboard(dashboardQuery())
	if err != nil {
		return nil, err
	}

	totalSeen, err := store.SeenCount()
	if err != nil {
		return nil, err
	}
	silentSeed := totalSeen == 0 && len(events) > 0 && boolEnv("FT_SEED_SILENT", true)

	subsRows, err := store.ListSubscribers()
	if err != nil {
		return nil, err
	}
	var subs []string
	for _, r := range subsRows {
		if e := strings.TrimSpace(r.Email); e != "" {
			subs = append(subs, e)
		}
	}

	var newOnes []filter.Event
	for _, ev := range events {
		isNew, err := store.IsNewEvent(ev.Key)
		if err != nil {
			return nil, err
		}
		if !isNew {
			continue
		}
		if silentSeed {
			if err := store.MarkEventSeen(ev.Key, seedStamp); err != nil {
				return nil, err
			}
			continue
		}
		newOnes = append(newOnes, ev)
		if err := store.MarkEventSeen(ev.Key, ""); err != nil {
			return nil, err
		}
	}

	if silentSeed {
		return map[string]interface{}{
			"source":     source,
			"scanned":    len(events),
			"silentSeed": true,
			"message":    "Premier passage : les événements actuels sont enregistrés sans email (évite le spam). Les prochains nouveaux déclencheront une notification.",
		}, nil
	}

	if len(newOnes) > 0 && len(subs) > 0 {
		for _, ev := range newOnes {
			if err := mail.SendNewEvent(subs, ev.Title, ev.StartAt, ev.LocationLabel, ev.URL); err != nil {
				log.Println("Erreur envoi mail pour", ev.Key, err)
			}
		}
	}

	notified := 0
	if len(subs) > 0 && len(newOnes) > 0 {
		notified = len(newOnes)
	}
	return map[string]interface{}{
		"source":      source,
		"scanned":     len(events),
		"newCount":    len(newOnes),
		"notified":    notified,
		"subscribers": len(subs),
		"silentSeed":  false,
	}, nil
}

// PollOnce interroge l’API, met à jour SQLite et envoie les mails si besoin.
func PollOnce(api *francetravail.API, store *dbstore.Store) (map[string]interface{}, error) {
	at := time.Now().UTC().Format(time.RFC3339Nano)
	out, err := pollOnceInternal(api, store)
	if err != nil {
		setLastErr(at, err)
		return nil, err
	}
	setLastOk(at, out)
	return out, nil
}

// StartScheduler lance le cron (expression POLL_CRON, défaut */30 * * * *).
func StartScheduler(api *francetravail.API, store *dbstore.Store) *cron.Cron {
	expr := strings.TrimSpace(os.Getenv("POLL_CRON"))
	if expr == "" {
		expr = "*/30 * * * *"
	}
	c := cron.New()
	_, err := c.AddFunc(expr, func() {
		if _, err := PollOnce(api, store); err != nil {
			log.Println("[poll] erreur", err)
		} else {
			log.Println("[poll]", time.Now().UTC().Format(time.RFC3339), "OK")
		}
	})
	if err != nil {
		log.Println("[poll] cron invalide, pas de planification:", err)
		return c
	}
	c.Start()
	if francetravail.IsMockMode() {
		log.Println("[poll] MOCK_FT=true — données fictives, emails réels si SMTP OK.")
	}
	log.Println("[poll] cron:", expr)
	return c
}
