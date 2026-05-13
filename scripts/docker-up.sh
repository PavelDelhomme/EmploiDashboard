#!/usr/bin/env bash
# Lance docker compose ; exporte HOST_PORT si AUTO_HOST_PORT=true (sans sourcer tout le .env).
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

auto="$(get_var AUTO_HOST_PORT true)"
start="$(get_var HOST_PORT_RANGE_START 3080)"
end="$(get_var HOST_PORT_RANGE_END 3200)"
cur="$(get_var HOST_PORT 3080)"

if [[ "$auto" == "true" ]]; then
  export HOST_PORT="$("$ROOT/scripts/free-port.sh" "$start" "$end" "$cur")"
  echo "[docker-up] HOST_PORT=${HOST_PORT} (AUTO_HOST_PORT=true, plage ${start}-${end})"
else
  export HOST_PORT="$cur"
  echo "[docker-up] HOST_PORT=${HOST_PORT} (AUTO_HOST_PORT=false)"
fi

exec docker compose up -d --build "$@"
