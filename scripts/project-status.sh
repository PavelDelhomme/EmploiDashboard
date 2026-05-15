#!/usr/bin/env bash
# Statut du projet EmploiDashboard : uniquement les conteneurs du compose « rennes-emploi » (ce dépôt).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

readonly R='\033[0;31m'
readonly G='\033[0;32m'
readonly Y='\033[1;33m'
readonly B='\033[0;34m'
readonly C='\033[0;36m'
readonly D='\033[1m'
readonly Z='\033[0m'

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

have_curl=0
if command -v curl >/dev/null 2>&1; then
  have_curl=1
fi

http_ok() {
  local url="$1"
  [[ "$have_curl" -eq 1 ]] || return 1
  curl -fsS -m 3 "$url" >/dev/null 2>&1
}

line_ok() { echo -e "  ${G}●${Z} $1"; }
line_ko() { echo -e "  ${R}●${Z} $1"; }
line_warn() { echo -e "  ${Y}●${Z} $1"; }
line_info() { echo -e "  ${C}○${Z} $1"; }

echo -e "${D}══ Rennes emploi dashboard ══${Z}"
echo -e "${B}Compose (name)${Z} : rennes-emploi — répertoire : ${ROOT}"
echo

if ! command -v docker >/dev/null 2>&1; then
  echo -e "${R}Docker non installé ou absent du PATH.${Z}"
  exit 1
fi

if [[ ! -f .env ]]; then
  echo -e "${Y}Pas de fichier .env (lance : make env).${Z}"
fi

echo -e "${D}Conteneurs (projet rennes-emploi uniquement)${Z}"
if docker compose version >/dev/null 2>&1; then
  docker compose ps -a
else
  echo -e "${R}docker compose indisponible.${Z}"
  exit 1
fi

echo
echo -e "${D}Étiquette Docker (filtre projet)${Z}"
docker ps -a --filter "label=com.docker.compose.project=rennes-emploi" --format "table {{.Names}}\t{{.Status}}\t{{.Ports}}" 2>/dev/null || true

host_port="$(get_var HOST_PORT "")"
mailpit_url="$(get_var MAILPIT_PUBLIC_URL "")"

echo
echo -e "${D}Ports attendus (.env)${Z}"
if [[ -n "$host_port" ]]; then
  line_info "HOST_PORT (dashboard hôte) : ${host_port}"
else
  line_ko "HOST_PORT non défini dans .env"
fi
if [[ -n "$mailpit_url" ]]; then
  line_info "MAILPIT_PUBLIC_URL : ${mailpit_url}"
else
  line_warn "MAILPIT_PUBLIC_URL vide — voir MAILPIT_UI_HOST_PORT dans .env"
fi

echo
echo -e "${D}Santé HTTP & URLs${Z}"
if [[ "$have_curl" -eq 0 ]]; then
  line_warn "curl absent — tests HTTP ignorés (installe curl pour les voyants vert/rouge)."
fi

if [[ -n "$host_port" ]]; then
  dash="http://127.0.0.1:${host_port}"
  if http_ok "${dash}/api/health"; then
    line_ok "Dashboard répond : ${dash}"
  else
    line_ko "Dashboard injoignable : ${dash} (conteneur arrêté ou port occupé par autre chose ?)"
  fi
fi

if [[ -n "$mailpit_url" ]]; then
  if http_ok "$mailpit_url"; then
    line_ok "Mailpit (interface) répond : ${mailpit_url}"
  else
    line_ko "Mailpit injoignable : ${mailpit_url}"
  fi
else
  mp="$(docker port rennes-emploi-mailpit 8025 2>/dev/null | head -1 | rev | cut -d: -f1 | rev || true)"
  if [[ -n "$mp" ]]; then
    u="http://127.0.0.1:${mp}"
    if http_ok "$u"; then
      line_ok "Mailpit répond (port publié) : $u"
    else
      line_ko "Mailpit : conteneur peut-être absent ou pas prêt — $u"
    fi
  else
    line_warn "Mailpit : pas de port publié détecté (make up lance le service mailpit)."
  fi
fi

cf_port="$(docker port rennes-emploi-camoufox 8765 2>/dev/null | head -1 | rev | cut -d: -f1 | rev || true)"
if [[ -n "$cf_port" ]]; then
  cfu="http://127.0.0.1:${cf_port}/health"
  if http_ok "$cfu"; then
    line_ok "Camoufox scraper : ${cfu}"
  else
    line_warn "Camoufox : port mappé ${cf_port} mais /health ne répond pas"
  fi
else
  line_info "Camoufox : non démarré (normal sans make up-full)."
fi

echo
echo -e "${D}Raccourcis${Z}"
echo -e "  ${B}make url${Z}          → URL dashboard si conteneur OK"
echo -e "  ${B}make url-mailpit${Z}  → URL boîte Mailpit"
echo -e "  ${B}make preview-port${Z} → port hôte prévu pour le dashboard"
echo
