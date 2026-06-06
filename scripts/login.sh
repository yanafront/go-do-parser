#!/usr/bin/env sh
set -e
cd "$(dirname "$0")/.."
export CGO_ENABLED=0
if [ -f .env ]; then
  set -a
  . ./.env
  set +a
fi
unset TG_SESSION
exec go run ./cmd/login "$@"
