#!/usr/bin/env bash
# Affiche le HOST_PORT utilisé par « make up » (sans lancer Docker).
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
  echo "[preview-port] HOST_PORT vide dans .env." >&2
  exit 1
fi

if [[ "$auto" != "true" ]]; then
  echo "AUTO_HOST_PORT=false → port fixe : ${cur}"
  echo "http://127.0.0.1:${cur}"
  exit 0
fi

p="$("$ROOT/scripts/free-port.sh" "$start" "$end" "$cur")"
echo "AUTO_HOST_PORT=true, plage ${start}-${end} → port choisi : ${p}"
echo "http://127.0.0.1:${p}"
