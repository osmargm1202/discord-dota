#!/usr/bin/env bash
# Verifica la API de Stratz con curl usando .env y datos de data/users.json y data/last_matches.json
# Uso: desde la raíz del proyecto: ./scripts/verify_stratz.sh

set -e
cd "$(dirname "$0")/.."

# Cargar .env (exportar variables)
if [ -f .env ]; then
  set -a
  source .env
  set +a
else
  echo "No existe .env. Copia .env.example y configura STRATZ_TOKEN."
  exit 1
fi

if [ -z "$STRATZ_TOKEN" ]; then
  echo "STRATZ_TOKEN no está definido en .env"
  exit 1
fi

API_URL="https://api.stratz.com/graphql"

# Obtener última partida: primer match_id de data/last_matches.json
# Formato: {"discord_id": match_id, ...} -> tomamos el primer valor numérico que sea match (10 dígitos)
LAST_MATCH_FILE="data/last_matches.json"
USERS_FILE="data/users.json"

if [ ! -f "$LAST_MATCH_FILE" ]; then
  echo "No existe $LAST_MATCH_FILE"
  exit 1
fi

# Extraer primer match_id (Dota match IDs son 10 dígitos; las claves son Discord IDs de 17+ dígitos)
MATCH_ID=$(grep -oE '\b[0-9]{10}\b' "$LAST_MATCH_FILE" | head -1)
if [ -z "$MATCH_ID" ]; then
  echo "No se pudo obtener match_id de $LAST_MATCH_FILE"
  exit 1
fi

# Extraer primer steam account_id de users.json (valores entre comillas, 9 dígitos)
if [ -f "$USERS_FILE" ]; then
  ACCOUNT_ID=$(grep -oE '"[0-9]{8,10}"' "$USERS_FILE" | head -1 | tr -d '"')
fi
if [ -z "$ACCOUNT_ID" ]; then
  ACCOUNT_ID="267273636"
fi

mkdir -p logs
echo "=== Verificación API Stratz ==="
echo "Match ID (última partida): $MATCH_ID"
echo "Steam Account ID (prueba): $ACCOUNT_ID"
echo ""

# 1) GetMatch - detalles de la partida
echo "--- 1) GetMatch (matchId: $MATCH_ID) ---"
curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $STRATZ_TOKEN" \
  -H "User-Agent: STRATZ_API" \
  -d "{\"query\":\"query GetMatch(\\\$matchId: Long!) { match(id: \\\$matchId) { id didRadiantWin durationSeconds startDateTime gameMode lobbyType radiantKills direKills parseStatus players { steamAccountId isRadiant heroId kills deaths assists level goldPerMinute experiencePerMinute heroDamage towerDamage heroHealing steamAccount { id name avatar isAnonymous } } } }\",\"variables\":{\"matchId\":$MATCH_ID}}" \
  | tee logs/stratz_get_match.json

echo ""
echo ""

# 2) GetPlayerRecentMatches - partidas recientes del jugador
echo "--- 2) GetPlayerRecentMatches (steamAccountId: $ACCOUNT_ID, take: 3) ---"
curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $STRATZ_TOKEN" \
  -H "User-Agent: STRATZ_API" \
  -d "{\"query\":\"query GetPlayerMatches(\\\$steamAccountId: Long!, \\\$take: Int!) { player(steamAccountId: \\\$steamAccountId) { matches(request: { take: \\\$take }) { id didRadiantWin durationSeconds startDateTime gameMode lobbyType radiantKills direKills players { steamAccountId isRadiant heroId kills deaths assists level steamAccount { id name avatar } } } } }\",\"variables\":{\"steamAccountId\":$ACCOUNT_ID,\"take\":3}}" \
  | tee logs/stratz_player_matches.json

echo ""
echo ""

# 3) GetPlayerProfile - perfil del jugador (para ver formato del avatar)
echo "--- 3) GetPlayerProfile (steamAccountId: $ACCOUNT_ID) ---"
# Usar comillas simples para que $steamAccountId sea literal en la query; solo ACCOUNT_ID se expande
curl -s -X POST "$API_URL" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $STRATZ_TOKEN" \
  -H "User-Agent: STRATZ_API" \
  -d '{"query":"query GetPlayer($steamAccountId: Long!) { player(steamAccountId: $steamAccountId) { steamAccountId steamAccount { name avatar isAnonymous } winCount matchCount } }","variables":{"steamAccountId":'$ACCOUNT_ID'}}' \
  | tee logs/stratz_get_player_profile.json

echo ""
echo "Respuestas guardadas en logs/stratz_get_match.json, logs/stratz_player_matches.json y logs/stratz_get_player_profile.json"
