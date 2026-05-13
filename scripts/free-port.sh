#!/usr/bin/env bash
# Affiche un port TCP libre sur 127.0.0.1 (bash /dev/tcp).
# Usage : free-port.sh [DEBUT] [FIN] [PORT_SOUHAITÉ]
set -euo pipefail

start="${1:-3080}"
end="${2:-3200}"
preferred="${3:-}"

port_in_use() {
  bash -c "exec 3<>/dev/tcp/127.0.0.1/$1" 2>/dev/null
}

if [[ -n "$preferred" ]] && [[ "$preferred" =~ ^[0-9]+$ ]] && ((preferred >= start && preferred <= end)) && ! port_in_use "$preferred"; then
  echo "$preferred"
  exit 0
fi

for ((p = start; p <= end; p++)); do
  if ! port_in_use "$p"; then
    echo "$p"
    exit 0
  fi
done

echo "Aucun port libre entre ${start} et ${end}" >&2
exit 1
