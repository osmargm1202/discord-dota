# Plan: Estadísticas adicionales en notificación de partida

Plan de implementación para añadir a la notificación de Discord: tipo de victoria (comeback/stomp/close), KP (kill participation), IMP (Individual Match Performance) y símbolos MVP / Top Core / Top Support. Basado en verificación con curl contra la API de Stratz (`.env` + `stratz.md`).

---

## 1. Verificación con la API (curl)

### 1.1 Tipo de victoria (analysisOutcome)

- **Campo:** `match.analysisOutcome` (tipo `MatchAnalysisOutcomeType`).
- **Valores del enum** (introspection):
  - `NONE`
  - `STOMPED`
  - `COMEBACK`
  - `CLOSE_GAME`
- **Campo adicional:** `match.predictedOutcomeWeight` (Byte, ej. 48–52) — peso/confianza del análisis (opcional para mostrar).
- **Ejemplo de respuesta:** `"analysisOutcome":"NONE"`, `"predictedOutcomeWeight":52`.

**Curl de prueba:**
```bash
source .env 2>/dev/null
curl -s -X POST "https://api.stratz.com/graphql" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $STRATZ_TOKEN" \
  -H "User-Agent: STRATZ_API" \
  -d '{"query":"query GetMatch($matchId: Long!) { match(id: $matchId) { id analysisOutcome predictedOutcomeWeight } }","variables":{"matchId":8672786803}}'
```

### 1.2 IMP (Individual Match Performance)

- **Campo:** `match.players[].imp` (Short).
- Valores observados: positivos (ej. 8, 20, 35) o negativos (ej. -30, -14).
- **Ejemplo:** `"imp":8`, `"imp":-30`.

### 1.3 Premios (MVP, Top Core, Top Support)

- **Campo:** `match.players[].award` (tipo `MatchPlayerAward`).
- **Valores del enum:**
  - `NONE`
  - `MVP`
  - `TOP_CORE`
  - `TOP_SUPPORT`
- **Ejemplo de respuesta:** `"award":"MVP"`, `"award":"TOP_CORE"`, `"award":"TOP_SUPPORT"`.

**Curl de prueba (players con imp y award):**
```bash
source .env 2>/dev/null
curl -s -X POST "https://api.stratz.com/graphql" \
  -H "Content-Type: application/json" \
  -H "Authorization: Bearer $STRATZ_TOKEN" \
  -H "User-Agent: STRATZ_API" \
  -d '{"query":"query GetMatch($matchId: Long!) { match(id: $matchId) { id radiantKills direKills players { steamAccountId isRadiant kills assists imp award } } }","variables":{"matchId":8672786803}}'
```

### 1.4 Kill participation (KP/KC)

- La API de Stratz **no** expone un campo `killParticipation` en `MatchPlayerType`.
- **Cálculo en código:** KP = (player.Kills + player.Assists) / teamTotalKills × 100.
- `teamTotalKills`: suma de `radiantKills` (Radiant) o suma de `direKills` (Dire) según `player.isRadiant`. Ya se tiene en `MatchResponse.RadiantScore` y `MatchResponse.DireScore`.

---

## 2. Cambios en el código

### 2.1 API / Stratz (`dota/stratz.go`)

| Paso | Descripción |
|------|-------------|
| 1 | Añadir a la query **GetMatch**: `analysisOutcome`, `predictedOutcomeWeight` (opcional). |
| 2 | Añadir a la query **GetMatch** en `players`: `imp`, `award`. |
| 3 | En **StratzMatch**: campos `AnalysisOutcome string`, `PredictedOutcomeWeight *int` (opcional). |
| 4 | En **StratzPlayer**: campos `Imp int` (Short), `Award string` (NONE, MVP, TOP_CORE, TOP_SUPPORT). |

### 2.2 Tipos internos (`dota/api.go`)

| Paso | Descripción |
|------|-------------|
| 1 | En **MatchResponse**: `AnalysisOutcome string`, `PredictedOutcomeWeight *int` (opcional). |
| 2 | En **Player**: `Imp int`, `Award string`. |
| 3 | En **StratzMatchToMatchResponse**: mapear `analysisOutcome`, `predictedOutcomeWeight`; en **StratzPlayerToPlayer**: mapear `imp`, `award`. |

### 2.3 Notificación (`discord/bot.go`)

| Paso | Descripción |
|------|-------------|
| 1 | **Tipo de victoria:** Si `match.AnalysisOutcome` no es `""` ni `NONE`, añadir un campo (o línea en descripción) con texto en español, ej.: `STOMPED` → "Stomp", `COMEBACK` → "Comeback", `CLOSE_GAME` → "Partida cerrada", `NONE` → no mostrar. Considerar perspectiva del jugador (ganó/perdió) para emoji o color si se desea. |
| 2 | **KP (Kill participation):** Calcular `teamKills := match.RadiantScore` o `match.DireScore` según `player.IsRadiant`. Si `teamKills > 0`: `kp := (player.Kills + player.Assists) * 100 / teamKills`. Mostrar en estadísticas como "KP: 72%" (junto a K/D/A o en campo propio). |
| 3 | **IMP:** Mostrar en estadísticas, ej. "IMP: +8" o "IMP: -10" (incluir signo si es negativo). |
| 4 | **Símbolo MVP / Top Core / Top Support:** Según `player.Award`: mostrar un símbolo o texto corto junto al nombre o en un campo. Ejemplos: MVP → "⭐ MVP", Top Core → "🏆 Core", Top Support → "🛡️ Support"; o iconos/shortcodes que prefieras. Solo mostrar si `award != "" && award != "NONE"`. |

### 2.4 Documentación

| Paso | Descripción |
|------|-------------|
| 1 | Actualizar **stratz.md**: documentar `analysisOutcome`, `predictedOutcomeWeight`, `players.imp`, `players.award` y el cálculo de KP en el cliente. |

---

## 3. Orden sugerido de implementación

1. **Stratz:** Añadir campos a la query GetMatch y a structs `StratzMatch` / `StratzPlayer`.
2. **dota/api.go:** Añadir campos a `MatchResponse` y `Player`; completar mapeo en `StratzMatchToMatchResponse` y `StratzPlayerToPlayer`.
3. **Notificación:**  
   - Calcular KP y mostrar KP + IMP en el embed.  
   - Añadir tipo de victoria (analysisOutcome) al embed.  
   - Añadir símbolo MVP / Top Core / Top Support según `player.Award`.
4. **stratz.md:** Actualizar con los nuevos campos y el cálculo de KP.

---

## 4. Resumen de datos verificados (curl)

| Dato | Origen | Notas |
|------|--------|--------|
| Tipo de victoria | `match.analysisOutcome` | NONE, STOMPED, COMEBACK, CLOSE_GAME |
| IMP | `match.players[].imp` | Short, puede ser negativo |
| Award | `match.players[].award` | NONE, MVP, TOP_CORE, TOP_SUPPORT |
| KP | Calculado | (kills + assists) / teamKills × 100; teamKills = RadiantScore o DireScore |

Endpoint: `POST https://api.stratz.com/graphql` con `Authorization: Bearer $STRATZ_TOKEN` (desde `.env`) y `User-Agent: STRATZ_API`.
