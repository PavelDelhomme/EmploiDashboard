#!/usr/bin/env bash
# Ajoute ou met à jour origin en SSH : git@github.com:USER/REPO.git
set -euo pipefail
USER="${1:?Usage: git-remote-ssh.sh GITHUB_USER REPO_NAME}"
REPO="${2:?Usage: git-remote-ssh.sh GITHUB_USER REPO_NAME}"
URL="git@github.com:${USER}/${REPO}.git"

if git remote get-url origin >/dev/null 2>&1; then
  git remote set-url origin "$URL"
  echo "origin → $URL"
else
  git remote add origin "$URL"
  echo "origin ajouté : $URL"
fi
