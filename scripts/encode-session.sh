#!/bin/sh
set -e
if [ -z "$1" ]; then
  echo "Usage: ./scripts/encode-session.sh path/to/session.json"
  exit 1
fi
if [ ! -f "$1" ]; then
  echo "File not found: $1"
  exit 1
fi
out="$(dirname "$0")/../tg_session.b64"
base64 < "$1" | tr -d '\n' > "$out"
len=$(wc -c < "$out" | tr -d ' ')
if base64 -D -i "$out" -o /tmp/tg_session_check.json 2>/dev/null || base64 -d -i "$out" -o /tmp/tg_session_check.json; then
  if diff -q "$1" /tmp/tg_session_check.json >/dev/null; then
    echo "OK: $out ($len bytes)"
    echo "Скопируйте в Railway:"
    echo "  pbcopy < $out"
    echo "или Railway CLI:"
    echo "  railway variables --set \"TG_SESSION=\$(cat $out)\""
    exit 0
  fi
fi
echo "ERROR: base64 check failed"
exit 1
