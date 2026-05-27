#!/bin/sh
set -e
if [ -z "$1" ]; then
  echo "Usage: ./scripts/encode-session.sh path/to/session.json"
  exit 1
fi
base64 < "$1" | tr -d '\n'
echo
