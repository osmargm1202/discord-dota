#!/usr/bin/env bash
# Corre el binario Nix contra los containers Docker locales (para pruebas)
# Uso: ./nix/run-local-test.sh
#
# Requiere:
#   1. docker compose -f docker-compose.yml -f docker-compose.local.yml up discord-dota-postgres discord-dota-minio -d
#   2. Un archivo .env con DISCORD_TOKEN, SERVER_ID, STRATZ_TOKEN, NOTIFICATION_CHANNEL_ID
#   3. nix build .# hecho (result/bin/dota-discord-bot)

set -e

SCRIPT_DIR="$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)"
ROOT="$SCRIPT_DIR/.."

if [ ! -f "$ROOT/.env" ]; then
  echo "ERROR: Falta $ROOT/.env — copia .env.example y llena las variables"
  exit 1
fi

if [ ! -f "$ROOT/result/bin/dota-discord-bot" ]; then
  echo "ERROR: Falta result/bin/dota-discord-bot — corre: nix build .#"
  exit 1
fi

# Cargar .env
set -a
source "$ROOT/.env"
set +a

# Sobreescribir DSN y MinIO para apuntar a Docker local (puertos del docker-compose.local.yml)
export POSTGRES_DSN="postgres://dotabot:${POSTGRES_PASSWORD:-changeme}@localhost:5435/dotabot?sslmode=disable"
export MINIO_ENDPOINT="localhost:9004"
export MINIO_ACCESS_KEY="${MINIO_ACCESS_KEY:-minioadmin}"
export MINIO_SECRET_KEY="${MINIO_SECRET_KEY:-changeme}"

echo "=== Corriendo bot Nix contra Docker local ==="
echo "  PG:    localhost:5435"
echo "  MinIO: localhost:9004"
echo "  Debug: ${DEBUG:-false}"
echo ""

exec "$ROOT/result/bin/dota-discord-bot" "$@"
