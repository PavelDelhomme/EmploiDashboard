#!/usr/bin/env bash
# Ajoute dans .env les clés présentes dans .env.example mais absentes de .env (ne modifie pas les valeurs existantes).
set -euo pipefail
ROOT="$(cd "$(dirname "$0")/.." && pwd)"
cd "$ROOT"

if [[ ! -f .env.example ]]; then
  echo "Fichier .env.example introuvable." >&2
  exit 1
fi

if [[ ! -f .env ]]; then
  cp .env.example .env
  echo "[merge-env] .env créé depuis .env.example."
  exit 0
fi

added=0
while IFS= read -r line || [[ -n "$line" ]]; do
  [[ "$line" =~ ^[[:space:]]*# ]] && continue
  t="${line#"${line%%[![:space:]]*}"}"
  t="${t%"${t##*[![:space:]]}"}"
  [[ -z "$t" ]] && continue
  [[ "$t" =~ ^([A-Za-z_][A-Za-z0-9_]*)= ]] || continue
  key="${BASH_REMATCH[1]}"
  if grep -qE "^[[:space:]]*${key}=" .env; then
    continue
  fi
  printf '%s\n' "$line" >> .env
  echo "[merge-env] ajouté : ${key}"
  added=$((added + 1))
done < .env.example

if [[ "$added" -eq 0 ]]; then
  echo "[merge-env] rien à ajouter (déjà à jour)."
else
  echo "[merge-env] terminé : ${added} ligne(s) ajoutée(s). Vérifie les valeurs vides à compléter."
fi
