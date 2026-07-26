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

AGENT_ID="${1:-}"
PHONE="${2:-}"
if [ -z "$AGENT_ID" ] || [ -z "$PHONE" ]; then
  echo "Usage: ./scripts/login-agent.sh <agent_id> <phone> [--fresh]"
  echo "Example: ./scripts/login-agent.sh a1 +375291112233"
  exit 1
fi
shift 2

export DATA_DIR="${DATA_DIR:-./data}"
exec go run ./cmd/login --agent "$AGENT_ID" --phone "$PHONE" "$@"
