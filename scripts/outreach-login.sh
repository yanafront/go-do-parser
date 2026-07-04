#!/usr/bin/env sh
set -e
cd "$(dirname "$0")/.."
export CGO_ENABLED=0
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi
unset OUTREACH_SESSION
export DATA_DIR="${OUTREACH_DATA_DIR:-./data/outreach}"
export TG_PHONE="${OUTREACH_PHONE:-$TG_PHONE}"
export TG_API_ID="${TG_API_ID:-$TG_API_ID}"
export TG_API_HASH="${TG_API_HASH:-$TG_API_HASH}"
if [ -z "$TG_PHONE" ]; then
  echo "OUTREACH_PHONE or TG_PHONE is required"
  exit 1
fi
exec go run ./cmd/login "$@"
