# Makefile — Rennes emploi dashboard
# Utilisation : make help
SHELL := /bin/bash
.DEFAULT_GOAL := help

ROOT := $(abspath .)

.PHONY: help env env-merge status up up-full down down-dashboard restart restart-full logs logs-full logs-mailpit ps ps-dashboard shell build build-full clean url url-full url-mailpit port-print preview-port install dev run \
	push push-all branches-init

help: ## Affiche les cibles disponibles
	@echo "Cibles principales :"
	@grep -E '^[a-zA-Z0-9_.-]+:.*##' $(MAKEFILE_LIST) | sort | sed 's/^\([^:]*\):.*## /  \1 → /'

status: ## État du projet (Docker rennes-emploi), ports, URLs, santé HTTP (couleurs)
	@chmod +x scripts/project-status.sh 2>/dev/null || true
	@bash scripts/project-status.sh

env: ## Crée .env depuis .env.example si absent (+ scripts exécutables)
	@test -f .env || cp .env.example .env
	@chmod +x scripts/*.sh 2>/dev/null || true
	@echo "[env] .env prêt. HOST_PORT = dashboard · MAILPIT_PUBLIC_URL = lire les mails locaux · make up."

url-mailpit: ## Affiche l’URL Mailpit (depuis .env ou port publié du conteneur)
	@u=$$(grep -E '^MAILPIT_PUBLIC_URL=' .env 2>/dev/null | tail -1 | cut -d= -f2- | tr -d '\"'); \
	if [[ -n "$$u" ]]; then echo "$$u"; else \
	  p=$$(docker port rennes-emploi-mailpit 8025 2>/dev/null | head -1 | rev | cut -d: -f1 | rev); \
	  if [[ -n "$$p" ]]; then echo "http://127.0.0.1:$$p"; else echo "[url-mailpit] Lance make up et définis MAILPIT_PUBLIC_URL dans .env"; exit 1; fi; \
	fi

env-merge: ## Ajoute dans .env les clés manquantes par rapport à .env.example (sans écraser)
	@chmod +x scripts/merge-env.sh 2>/dev/null || true
	@bash scripts/merge-env.sh

up: env ## Démarre Docker — ports = HOST_PORT et CAMOUFOX_HOST_PORT dans .env (fixes)
	@chmod +x scripts/*.sh
	@bash scripts/docker-up.sh

up-full: env ## Démarre Docker — dashboard + Camoufox (COMPOSE_WITH_CAMOUFOX=true)
	@chmod +x scripts/*.sh
	@COMPOSE_WITH_CAMOUFOX=true bash scripts/docker-up.sh

down: ## Arrête les conteneurs (y compris ceux du profil camoufox si actifs)
	docker compose --profile camoufox down

down-dashboard: ## Arrête sans le profil camoufox (alias compose down minimal)
	docker compose down

restart: down up ## Redémarre le dashboard seul

restart-full: down up-full ## Redémarre dashboard + Camoufox

logs: ## Suit les logs du conteneur dashboard
	docker compose logs -f dashboard

logs-full: ## Logs dashboard + camoufox-scraper (profil camoufox)
	docker compose --profile camoufox logs -f dashboard camoufox-scraper

logs-mailpit: ## Logs du service Mailpit (capture SMTP)
	docker compose logs -f mailpit

ps: ## État des conteneurs (inclut camoufox si profil utilisé au up)
	docker compose --profile camoufox ps -a

ps-dashboard: ## État sans forcer le profil camoufox
	docker compose ps -a

shell: ## Shell sh dans le conteneur
	docker exec -it rennes-emploi-dashboard sh

build: env ## Build l’image dashboard sans lancer
	docker compose build

build-full: env ## Build dashboard + image camoufox-scraper
	docker compose --profile camoufox build

clean: down ## Arrête et rappelle comment supprimer les volumes (SQLite + boîte Mailpit)
	@echo "Volumes nommés : rennes-emploi-sqlite (dashboard), rennes-emploi-mailpit (mails captés)."
	@echo "Supprimer : docker volume rm rennes-emploi-sqlite rennes-emploi-mailpit"

url: ## Affiche l’URL si le conteneur tourne (port publié)
	@p=$$(docker port rennes-emploi-dashboard 3000 2>/dev/null | head -1 | rev | cut -d: -f1 | rev); \
	if [[ -n "$$p" ]]; then echo "http://127.0.0.1:$$p"; else echo "[url] Conteneur absent — lance : make up"; exit 1; fi

url-full: ## URL dashboard + URL scraper Camoufox si les conteneurs tournent
	@p=$$(docker port rennes-emploi-dashboard 3000 2>/dev/null | head -1 | rev | cut -d: -f1 | rev); \
	if [[ -n "$$p" ]]; then echo "Dashboard : http://127.0.0.1:$$p"; else echo "[url-full] Dashboard absent — lance : make up-full"; exit 1; fi
	@cp=$$(docker port rennes-emploi-camoufox 8765 2>/dev/null | head -1 | rev | cut -d: -f1 | rev); \
	if [[ -n "$$cp" ]]; then echo "Camoufox  : http://127.0.0.1:$$cp/health"; else echo "Camoufox  : (conteneur absent — make up-full + build camoufox)"; fi

port-print: ## Affiche HOST_PORT lu dans .env (sans test réseau)
	@grep -E '^HOST_PORT=' .env 2>/dev/null || echo "Pas de .env"

preview-port: env ## Affiche HOST_PORT + URL (comme au prochain « make up » ; AUTO_HOST_PORT=true = scan plage)
	@chmod +x scripts/preview-host-port.sh
	@bash scripts/preview-host-port.sh

install: env ## npm install local (hors Docker, legacy Node)
	@command -v npm >/dev/null && (cd "$(ROOT)" && npm install) || { echo "[install] npm introuvable dans ce shell — utilise Docker : make up"; exit 1; }

dev: install ## Lance le serveur Node en local (legacy ; backend actuel : Go + make run)
	@command -v npm >/dev/null && (cd "$(ROOT)" && npm run dev) || exit 1

run: env ## Lance le serveur Go en local (.env chargé par l’application)
	@command -v go >/dev/null || { echo "[run] go introuvable"; exit 1; }
	@cd "$(ROOT)" && go run ./cmd/rennes-emploi

# --- Git (optionnel) ---
push push-all: ## Pousse main, develop et feature/tooling-makefile-ports vers origin
	git push -u origin main
	-git push -u origin develop
	-git push -u origin feature/tooling-makefile-ports

branches-init: ## Affiche le modèle de branches (main / develop / feature)
	@echo "Modèle : main (stable) ← develop (intégration) ← feature/* (travail)"
	@echo "Branche courante : $$(git branch --show-current 2>/dev/null || echo ?)"
