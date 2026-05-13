# Makefile — Rennes emploi dashboard
# Utilisation : make help
SHELL := /bin/bash
.DEFAULT_GOAL := help

ROOT := $(abspath .)

.PHONY: help env up down restart logs ps shell build clean url port-print install dev \
	push push-all branches-init

help: ## Affiche les cibles disponibles
	@echo "Cibles principales :"
	@grep -E '^[a-zA-Z0-9_.-]+:.*##' $(MAKEFILE_LIST) | sort | sed 's/^\([^:]*\):.*## /  \1 → /'

env: ## Crée .env depuis .env.example si absent (+ scripts exécutables)
	@test -f .env || cp .env.example .env
	@chmod +x scripts/*.sh 2>/dev/null || true
	@echo "[env] .env prêt (édite FT_* / SMTP_*). AUTO_HOST_PORT=true par défaut."

up: env ## Démarre Docker (HOST_PORT auto dans la plage si AUTO_HOST_PORT=true)
	@chmod +x scripts/*.sh
	@bash scripts/docker-up.sh

down: ## Arrête les conteneurs
	docker compose down

restart: down up ## Redémarre la stack

logs: ## Suit les logs du conteneur dashboard
	docker compose logs -f dashboard

ps: ## État docker compose
	docker compose ps -a

shell: ## Shell sh dans le conteneur
	docker exec -it rennes-emploi-dashboard sh

build: env ## Build l’image sans lancer
	docker compose build

clean: down ## Arrête et affiche comment supprimer le volume SQLite
	@echo "Pour effacer la base locale : docker volume rm rennes-emploi-sqlite"

url: ## Affiche l’URL si le conteneur tourne (port publié)
	@p=$$(docker port rennes-emploi-dashboard 3000 2>/dev/null | head -1 | rev | cut -d: -f1 | rev); \
	if [[ -n "$$p" ]]; then echo "http://127.0.0.1:$$p"; else echo "[url] Conteneur absent — lance : make up"; exit 1; fi

port-print: ## Affiche HOST_PORT lu dans .env (sans test réseau)
	@grep -E '^HOST_PORT=' .env 2>/dev/null || echo "Pas de .env"

install: env ## npm install local (hors Docker)
	@command -v npm >/dev/null && (cd "$(ROOT)" && npm install) || { echo "[install] npm introuvable dans ce shell — utilise Docker : make up"; exit 1; }

dev: install ## Lance le serveur Node en local (PORT depuis .env)
	@command -v npm >/dev/null && (cd "$(ROOT)" && npm run dev) || exit 1

# --- Git (optionnel) ---
push push-all: ## Pousse main, develop et feature/tooling-makefile-ports vers origin
	git push -u origin main
	-git push -u origin develop
	-git push -u origin feature/tooling-makefile-ports

branches-init: ## Affiche le modèle de branches (main / develop / feature)
	@echo "Modèle : main (stable) ← develop (intégration) ← feature/* (travail)"
	@echo "Branche courante : $$(git branch --show-current 2>/dev/null || echo ?)"
