#!/usr/bin/env bash
# Lance docker compose ; exporte HOST_PORT depuis .env (sans sourcer tout le .env).
# AUTO_HOST_PORT=true : comportement optionnel (scan d'une plage) — déconseillé si tu veux des ports réservés à ce projet.
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env ]]; then
  echo "Fichier .env absent — lance : make env" >&2
  exit 1
fi

get_var() {
  local key="$1" def="${2:-}"
  local line
  line=$(grep -E "^[[:space:]]*${key}=" .env 2>/dev/null | tail -1 || true)
  [[ -z "$line" ]] && { echo "$def"; return; }
  line="${line#*=}"
  line="${line%%#*}"
  line="${line%"${line##*[![:space:]]}"}"
  line="${line#"${line%%[![:space:]]*}"}"
  line="${line%\"}"
  line="${line#\"}"
  line="${line%\'}"
  line="${line#\'}"
  echo "$line"
}

auto="$(get_var AUTO_HOST_PORT false)"
start="$(get_var HOST_PORT_RANGE_START 18080)"
end="$(get_var HOST_PORT_RANGE_END 18999)"
cur="$(get_var HOST_PORT "")"

if [[ -z "$cur" ]]; then
  echo "[docker-up] Erreur : HOST_PORT vide dans .env. Définis un port dédié à ce projet (ex. HOST_PORT=18443)." >&2
  exit 1
fi

if [[ "$auto" == "true" ]]; then
  export HOST_PORT="$("$ROOT/scripts/free-port.sh" "$start" "$end" "$cur")"
  echo "[docker-up] HOST_PORT=${HOST_PORT} (AUTO_HOST_PORT=true, plage ${start}-${end})"
else
  export HOST_PORT="$cur"
  echo "[docker-up] HOST_PORT=${HOST_PORT} (fixe depuis .env — vérifier qu'il ne chevauche pas d'autres stacks Docker)"
fi

# Stack complète avec Camoufox : COMPOSE_WITH_CAMOUFOX=true (ex. make up-full)
compose_extra=()
if [[ "${COMPOSE_WITH_CAMOUFOX:-}" == "true" ]]; then
  compose_extra=(--profile camoufox)
  echo "[docker-up] profil compose : camoufox (service camoufox-scraper)"
fi

exec docker compose "${compose_extra[@]}" up -d --build "$@"
