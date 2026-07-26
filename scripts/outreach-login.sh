#!/usr/bin/env sh
set -e
cd "$(dirname "$0")/.."

resolve_go() {
  if command -v go >/dev/null 2>&1; then
    command -v go
    return
  fi
  for candidate in \
    "$HOME/go/bin/go" \
    /usr/local/go/bin/go \
    /opt/homebrew/bin/go \
    /usr/local/bin/go
  do
    if [ -x "$candidate" ]; then
      echo "$candidate"
      return
    fi
  done
  echo "go not found. Add Go to PATH or install from https://go.dev/dl/" >&2
  exit 1
}

GO_BIN="$(resolve_go)"
export PATH="$(dirname "$GO_BIN"):$PATH"
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
exec "$GO_BIN" run ./cmd/login "$@"
