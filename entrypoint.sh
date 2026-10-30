#!/bin/sh
PORT="${PORT:-10000}"
HOST="${KIROCC_HOST:-0.0.0.0}"
DB_PATH="${KIROCC_DB_PATH:-/app/data.sqlite3}"

# 1. If KIROCC_DB_BASE64 env var is provided
if [ -n "$KIROCC_DB_BASE64" ]; then
    echo "Decoding KIROCC_DB_BASE64 to $DB_PATH..."
    echo "$KIROCC_DB_BASE64" | base64 -d > "$DB_PATH"
# 2. If secret file data.sqlite3.b64 was provided
elif [ -f "/etc/secrets/data.sqlite3.b64" ]; then
    echo "Decoding /etc/secrets/data.sqlite3.b64 to $DB_PATH..."
    base64 -d /etc/secrets/data.sqlite3.b64 > "$DB_PATH"
# 3. If raw secret file data.sqlite3 was provided
elif [ -f "/etc/secrets/data.sqlite3" ]; then
    DB_PATH="/etc/secrets/data.sqlite3"
fi

echo "Starting kirocc on ${HOST}:${PORT} with DB ${DB_PATH}..."
exec /app/kirocc -host "${HOST}" -port "${PORT}" -db "${DB_PATH}" "$@"
