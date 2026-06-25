#!/usr/bin/env bash
# Prueba local 100% Nix — sin Docker.
# Arranca PostgreSQL + MinIO como procesos temporales en /tmp,
# corre el bot, y limpia todo al salir.
#
# Uso:
#   nix develop .# --command ./nix/run-local-test.sh
#
# Requiere .env con: DISCORD_TOKEN, SERVER_ID, STRATZ_TOKEN, NOTIFICATION_CHANNEL_ID

set -e
ROOT="$(cd "$(dirname "${BASH_SOURCE[0]}")/.." && pwd)"

# ---- Validaciones ----
if [ ! -f "$ROOT/.env" ]; then
  echo "ERROR: Falta .env — copia .env.example y llena DISCORD_TOKEN, SERVER_ID, STRATZ_TOKEN, NOTIFICATION_CHANNEL_ID"
  exit 1
fi
for cmd in postgres minio initdb createdb createuser; do
  if ! command -v "$cmd" &>/dev/null; then
    echo "ERROR: '$cmd' no encontrado. Corre dentro de: nix develop .#"
    exit 1
  fi
done

# ---- Directorios temporales ----
TMPDIR="$(mktemp -d /tmp/dota-local-XXXXXX)"
PG_DIR="$TMPDIR/pg"
MINIO_DIR="$TMPDIR/minio"
PG_SOCKET="$TMPDIR/pg-socket"
MINIO_PORT=19000
PG_PORT=15432

mkdir -p "$PG_DIR" "$MINIO_DIR" "$PG_SOCKET"

# ---- Limpieza al salir ----
cleanup() {
  echo ""
  echo "==> Limpiando procesos locales..."
  [ -n "$BOT_PID" ]   && kill "$BOT_PID"   2>/dev/null || true
  [ -n "$MINIO_PID" ] && kill "$MINIO_PID" 2>/dev/null || true
  [ -n "$PG_PID" ]    && kill "$PG_PID"    2>/dev/null || true
  wait 2>/dev/null || true
  rm -rf "$TMPDIR"
  echo "==> Limpieza completa."
}
trap cleanup EXIT INT TERM

# ---- PostgreSQL local ----
echo "==> Iniciando PostgreSQL local en puerto $PG_PORT..."
initdb -D "$PG_DIR" --auth=trust --username=dotabot -q
cat >> "$PG_DIR/postgresql.conf" <<EOF
port = $PG_PORT
unix_socket_directories = '$PG_SOCKET'
listen_addresses = '127.0.0.1'
EOF
postgres -D "$PG_DIR" -k "$PG_SOCKET" &
PG_PID=$!

# Esperar que PG acepte conexiones
for i in $(seq 1 15); do
  pg_isready -h 127.0.0.1 -p "$PG_PORT" -U dotabot &>/dev/null && break
  sleep 1
done
pg_isready -h 127.0.0.1 -p "$PG_PORT" -U dotabot || { echo "ERROR: PostgreSQL no arrancó"; exit 1; }

createdb -h 127.0.0.1 -p "$PG_PORT" -U dotabot dotabot
echo "==> PostgreSQL listo."

# ---- MinIO local ----
echo "==> Iniciando MinIO local en puerto $MINIO_PORT..."
MINIO_ROOT_USER=minioadmin \
MINIO_ROOT_PASSWORD=minioadmin \
minio server "$MINIO_DIR" --address "127.0.0.1:$MINIO_PORT" --console-address "127.0.0.1:$((MINIO_PORT+1))" &>/tmp/minio-local.log &
MINIO_PID=$!
sleep 3
kill -0 "$MINIO_PID" 2>/dev/null || { echo "ERROR: MinIO no arrancó. Log:"; cat /tmp/minio-local.log; exit 1; }
echo "==> MinIO listo."

# ---- Cargar .env y sobreescribir con valores locales ----
set -a
source "$ROOT/.env"
set +a

export POSTGRES_DSN="postgres://dotabot@127.0.0.1:$PG_PORT/dotabot?sslmode=disable"
export MINIO_ENDPOINT="127.0.0.1:$MINIO_PORT"
export MINIO_ACCESS_KEY="minioadmin"
export MINIO_SECRET_KEY="minioadmin"
export MINIO_PUBLIC_URL="http://127.0.0.1:$MINIO_PORT"
export DEBUG="${DEBUG:-true}"

echo ""
echo "=== Bot corriendo (Nix, sin Docker) ==="
echo "  PG:    127.0.0.1:$PG_PORT"
echo "  MinIO: 127.0.0.1:$MINIO_PORT"
echo "  Ctrl+C para salir y limpiar"
echo ""

"$ROOT/result/bin/dota-discord-bot" "$@"
