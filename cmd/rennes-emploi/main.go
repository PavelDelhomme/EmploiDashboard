package main

import (
	"log"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/joho/godotenv"

	"rennes-emploi-dashboard/internal/dbstore"
	"rennes-emploi-dashboard/internal/francetravail"
	"rennes-emploi-dashboard/internal/poll"
	"rennes-emploi-dashboard/internal/scraperclient"
	"rennes-emploi-dashboard/internal/web"
)

func startupPollEnabled() bool {
	v := strings.ToLower(strings.TrimSpace(os.Getenv("STARTUP_POLL")))
	if v == "" {
		return true
	}
	return v != "false"
}

func main() {
	_ = godotenv.Load()

	store, err := dbstore.Open()
	if err != nil {
		log.Fatal(err)
	}
	defer store.Close()

	api := francetravail.New()
	if sc := scraperclient.FromEnv(); sc != nil {
		api.Scraper = sc
		log.Println("Fusion Camoufox activée (CAMOUFOX_SCRAPER_URL)")
	}
	poll.StartScheduler(api, store)

	if startupPollEnabled() {
		time.AfterFunc(3*time.Second, func() {
			if _, err := poll.PollOnce(api, store); err != nil {
				log.Println("startup poll:", err)
			} else {
				log.Println("startup poll OK")
			}
		})
	}

	port := strings.TrimSpace(os.Getenv("PORT"))
	if port == "" {
		port = "3000"
	}

	srv := &web.Server{
		Store:     store,
		API:       api,
		PublicDir: strings.TrimSpace(os.Getenv("PUBLIC_DIR")),
	}

	log.Printf("HTTP http://0.0.0.0:%s", port)
	log.Fatal(http.ListenAndServe("0.0.0.0:"+port, srv.Handler()))
}
