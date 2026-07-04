#!/usr/bin/env sh
set -e
if [ -z "$1" ]; then
  echo "Usage: ./scripts/encode-outreach-session.sh path/to/session.json"
  exit 1
fi
out="$(dirname "$0")/../outreach_session.b64"
base64 < "$1" | tr -d '\n' > "$out"
echo "OK: $out"
echo "Railway: OUTREACH_SESSION=\$(cat $out)"
