#!/usr/bin/env bash

set -euo pipefail

if [ $# -ne 1 ]; then
  echo "Usage: $0 <migration_name>"
  echo "Example: $0 create_users"
  exit 1
fi

NAME=$1
MIGRATIONS_DIR="internal/db/migrations"

command -v migrate >/dev/null 2>&1 || {
  echo "❌ migrate CLI not found. Install it first."
  exit 1
}

mkdir -p "$MIGRATIONS_DIR"

migrate create \
  -seq \
  -ext sql \
  -dir "$MIGRATIONS_DIR" \
  "$NAME"

echo "✅ Migration created:"
ls -1 "$MIGRATIONS_DIR" | tail -n 2
