# API Stratz en Go – Uso y solicitudes

Documentación del uso de la API GraphQL de Stratz en este proyecto: cómo se hacen las solicitudes, qué resultados devuelve y cómo verificar con curl usando `.env` y los datos de `data/users.json` y `data/last_matches.json`.

## Endpoint y autenticación

- **URL:** `https://api.stratz.com/graphql`
- **Método:** POST
- **Headers:**
  - `Content-Type: application/json`
  - `Authorization: Bearer <STRATZ_TOKEN>`
  - `User-Agent: STRATZ_API`

El token se obtiene de la variable de entorno `STRATZ_TOKEN` (configurada en `.env`). Sin token la API no responde o devuelve error.

## Cómo se hacen las solicitudes en Go

En `dota/stratz.go`:

1. **Cliente:** `NewStratzClient(token string)` crea un `StratzClient` con timeout 15s.
2. **Todas las llamadas pasan por** `makeRequest(query, variables, result)`:
   - Construye un JSON `{"query": "...", "variables": {...}}`.
   - POST a `https://api.stratz.com/graphql` con los headers anteriores.
   - Lee el body y deserializa `{"data": ..., "errors": [...]}`.
   - Si hay `errors`, devuelve error; si no, unmarshal de `data` en `result`.
3. **Modo debug:** Si `cfg.Debug` está activo (o `--debug`), se llama `stratzClient.SetDebug(true)` y cada request/response se escribe en `logs/stratz_debug.log` (query, variables y respuesta raw).

Para ver en desarrollo exactamente qué pide y qué devuelve la API, ejecutar con `DEBUG=true` en `.env` o con el flag `--debug`.

## Queries y mutaciones usadas

### 1. GetMatch – Detalle de una partida

**Uso en Go:** `stratzClient.GetMatch(matchID int64) (*StratzMatch, error)`

**Variables:** `matchId` (Long, ID de partida de Dota 2).

**Campos solicitados:**

- `match`: `id`, `didRadiantWin`, `durationSeconds`, `startDateTime`, `gameMode`, `lobbyType`, `radiantKills`, `direKills`, `parsedDateTime`, `topLaneOutcome`, `midLaneOutcome`, `bottomLaneOutcome`
- `match.players`: `steamAccountId`, `isRadiant`, `heroId`, `lane`, `role`, `kills`, `deaths`, `assists`, `level`, `goldPerMinute`, `experiencePerMinute`, `heroDamage`, `towerDamage`, `heroHealing`, `steamAccount { id, name, avatar, isAnonymous }`

**Partida parseada (MatchType.parsedDateTime):** Stratz expone `parsedDateTime: Long`. Si la API devuelve un valor > 0, la partida está parseada; si devuelve `null` o no está presente, no está parseada. `IsMatchParsed` en código usa solo este campo (sin fallback). La variable de entorno `PARSED=true` (por defecto) hace que solo se envíe notificación cuando la partida esté parseada; `PARSED=false` notifica cualquier partida nueva sin verificar parse.

**Lane outcomes (LaneOutcomeEnums):** `TIE`, `RADIANT_VICTORY`, `RADIANT_STOMP`, `DIRE_VICTORY`, `DIRE_STOMP`. La API devuelve strings, ej. `"topLaneOutcome":"RADIANT_VICTORY"`, `"midLaneOutcome":"RADIANT_VICTORY"`, `"bottomLaneOutcome":"TIE"`. Se usan en la notificación para el resultado de cada línea y victoria/derrota en fase de línea (jungle/roaming no se marca).

**Resultado típico (verificado con curl usando .env y matchId de data/last_matches.json):**

- `gameMode` / `lobbyType`: vienen como enum string, ej. `"ALL_PICK_RANKED"`, `"RANKED"`. El código usa `stratzIntOrStr` para aceptar ambos.
- `radiantKills` / `direKills`: la API devuelve **array** de kills por minuto; el código usa `stratzIntOrArray` y suma.
- `parsedDateTime`: Long (timestamp); null si no está parseada.
- `topLaneOutcome`, `midLaneOutcome`, `bottomLaneOutcome`: strings `"RADIANT_VICTORY"`, `"DIRE_VICTORY"`, `"TIE"`, etc.
- `players[].lane`: `"SAFE_LANE"`, `"OFF_LANE"`, `"MID_LANE"`. `players[].role`: `"CORE"`, `"HARD_SUPPORT"`, `"LIGHT_SUPPORT"`, etc.

### 2. GetPlayerRecentMatches – Partidas recientes de un jugador

**Uso en Go:** `stratzClient.GetPlayerRecentMatches(steamAccountID int64, limit int) ([]StratzMatch, error)`

**Variables:** `steamAccountId` (Long), `take` (Int).

**Límite:** La API de Stratz impone un **máximo de 100** en `take` para `player.matches(request: { take })`. Si se supera, devuelve error: "You have surpassed the maximum take value of: 100".

**Campos:** Igual que en GetMatch para cada partida (id, didRadiantWin, durationSeconds, players con steamAccount, etc.). El detalle completo (parsedDateTime, lane outcomes, etc.) se pide con GetMatch cuando hay nueva partida.

**Resultado:** Array de partidas; cada una tiene la misma estructura que `StratzMatch` (players, scores, etc.).

### 3. GetPlayerProfile – Perfil del jugador

**Uso en Go:** `stratzClient.GetPlayerProfile(steamAccountID int64) (*StratzPlayerStats, error)`

**Variables:** `steamAccountId` (Long).

**Campos:** `player.steamAccountId`, `player.steamAccount { name, avatar, isAnonymous }`, `player.winCount`, `player.matchCount`, `player.ranks(seasonRankIds: [0]) { rankBracket }` (si el schema lo expone).

**Formato del avatar (Stratz):** Stratz devuelve `steamAccount.avatar` como **URL completa** en tamaño full, por ejemplo:
`https://avatars.steamstatic.com/d84c21a60756feb8a22d2e505af06cf810e147ad_full.jpg`
No es ruta relativa; es el mismo formato que la API de Steam (CDN `avatars.steamstatic.com`, sufijo `_full.jpg` para 184×184 px).

**Resultado:** Nombre, avatar, W/L total, rank (si está disponible). En código, el avatar se pasa por `NormalizeSteamAvatarURL`: si Stratz devuelve URL completa (como arriba), se usa tal cual; si en algún caso viniera ruta relativa o URL pequeña (sin `_full`), se añade base CDN de Steam y/o sufijo `_full.jpg` para usarlo en la notificación de Discord.

### 4. GetPlayerWinLoss – Victorias/derrotas (últimas N partidas)

**Uso en Go:** `stratzClient.GetPlayerWinLoss(steamAccountID int64, limit int, heroID int) (*WinLossResponse, error)`

**Variables:** `steamAccountId`, `take`; si `heroID > 0` también `heroId` y la query filtra por ese héroe.

**Campos:** Por partida solo `didRadiantWin` y `players(steamAccountId: ...) { isRadiant }` para calcular si el jugador ganó o perdió.

**Resultado:** `WinLossResponse { Win, Lose }` calculado en Go a partir de las partidas.

### 5. GetMultiplePlayersWinLoss – W/L de varios jugadores

**Uso en Go:** `stratzClient.GetMultiplePlayersWinLoss(steamAccountIDs []int64, limit int) (map[int64]*WinLossResponse, error)`

Varias queries alias en una sola petición (`player0`, `player1`, …) con la misma estructura que GetPlayerWinLoss. Resultado: mapa por `steamAccountId` con Win/Lose.

### 6. RequestParseMatch – Solicitar parse de una partida

**Uso en Go:** `stratzClient.RequestParseMatch(matchID int64) error`

**Mutación:** `requestParse(matchId: Long!)`. Variables: `matchId`.

**Resultado:** La API puede devolver `true`/`false`. Si devuelve `false` o error, el código propaga error. Sirve para partidas recién jugadas que aún no están parseadas (parsedDateTime null).

## Estructuras Go principales (resultados)

- **StratzMatch:** id, DidRadiantWin, DurationSeconds, StartDateTime, GameMode, LobbyType, RadiantKills, DireKills, ParsedDateTime, Players.
- **StratzPlayer:** SteamAccountID, IsRadiant, HeroID, Kills, Deaths, Assists, Level, GoldPerMinute, ExperiencePerMinute, HeroDamage, TowerDamage, HeroHealing, SteamAccount (id, name, avatar, isAnonymous). Lane/Role si la API los expone en la query.
- **StratzPlayerStats:** SteamAccountID, Name, Avatar, WinCount, MatchCount, RankBracket.

Los tipos `stratzIntOrStr` y `stratzIntOrArray` permiten que la API devuelva enums/strings o números y arrays sin romper el unmarshal.

## Verificación con curl y datos del proyecto

1. **Variables:** Cargar `.env` (incluye `STRATZ_TOKEN`). Datos de prueba:
   - **Última partida:** primer match ID de 10 dígitos en `data/last_matches.json` (p. ej. `8671239987`, `8671320666`).
   - **Jugador de prueba:** primer steam account_id en `data/users.json` (p. ej. `267273636`).

2. **Script incluido:**  
   Desde la raíz del proyecto:
   ```bash
   chmod +x scripts/verify_stratz.sh
   ./scripts/verify_stratz.sh
   ```
   El script:
   - Carga `.env` y comprueba `STRATZ_TOKEN`.
   - Lee match ID de `data/last_matches.json` y account ID de `data/users.json`.
   - Hace POST a Stratz con:
     - GetMatch(matchId) → respuesta en `logs/stratz_get_match.json`.
     - GetPlayerRecentMatches(steamAccountId, take: 3) → respuesta en `logs/stratz_player_matches.json`.

3. **Verificar GetMatch con curl (usando .env y lane outcomes):**  
   No incluir `parseStatus` en la query: ya no existe en el schema de Stratz y devuelve 500. Payload de ejemplo en `logs/stratz_get_match_payload.json`; respuesta en `logs/stratz_get_match_response.json`.
   ```bash
   source .env 2>/dev/null
   curl -s -X POST "https://api.stratz.com/graphql" \
     -H "Content-Type: application/json" \
     -H "Authorization: Bearer $STRATZ_TOKEN" \
     -H "User-Agent: STRATZ_API" \
     -d @logs/stratz_get_match_payload.json
   ```
   Respuesta típica incluye `topLaneOutcome`, `midLaneOutcome`, `bottomLaneOutcome` (strings: RADIANT_VICTORY, DIRE_VICTORY, TIE, etc.) y `players[].lane` / `players[].role`.

## Resumen para modificar el proyecto

- Añadir/quitar campos: editar la string `query` en la función correspondiente de `dota/stratz.go` (GetMatch, GetPlayerRecentMatches, etc.) y, si hace falta, los structs de resultado y los tipos `StratzMatch`/`StratzPlayer`/etc.
- Probar la API sin levantar el bot: `DEBUG=true` o `--debug` y revisar `logs/stratz_debug.log`, o ejecutar `./scripts/verify_stratz.sh` y revisar `logs/stratz_get_match.json` y `logs/stratz_player_matches.json`.
- Nuevas queries: usar `makeRequest(query, variables, &result)` con el mismo endpoint y headers; opcionalmente añadir un método en `StratzClient` que prepare query/variables y llame a `makeRequest`.
