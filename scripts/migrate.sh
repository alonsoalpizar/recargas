#!/usr/bin/env bash
set -euo pipefail

DB="${1:-recargas}"
DIR="$(cd "$(dirname "$0")/.." && pwd)"

for f in "$DIR"/migrations/*.sql; do
  echo ">> applying $(basename "$f") to $DB"
  psql -h 127.0.0.1 -U recargas -d "$DB" -v ON_ERROR_STOP=1 -f "$f"
done

echo "OK"
