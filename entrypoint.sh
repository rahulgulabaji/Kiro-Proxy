#!/bin/sh
PORT="${PORT:-10000}"
HOST="${KIROCC_HOST:-0.0.0.0}"
DB_PATH="${KIROCC_DB_PATH:-/etc/secrets/data.sqlite3}"

echo "Starting kirocc on ${HOST}:${PORT} with DB ${DB_PATH}..."
exec /app/kirocc -host "${HOST}" -port "${PORT}" -db "${DB_PATH}" "$@"

