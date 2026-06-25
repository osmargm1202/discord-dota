# Agent Guidelines — Dota Discord Bot

## Stratz API Limits (Default/Free Token)

The project uses a single Stratz default token (cannot generate additional tokens).

| Limit     | Value  |
|-----------|--------|
| /second   | 20     |
| /minute   | 250    |
| /hour     | 2,000  |
| /day      | 10,000 |

**IP Lock**: The default token is locked to the IP it was first used from.
- Production server (`server.fifrex.com`) = valid IP
- Local dev from a different IP = 403 error — expected, not a bug
- `BACKFILL_DELAY_MS=700` keeps backfill under 85 calls/min (safe margin)

**Budget guidance**:
- Polling every minute × 3 users = ~3–6 calls/min (match check + profile)
- Backfill 3 users × 3 pages = ~9 calls total (runs once, idempotent)
- `/dota stats` = 1–3 calls per use
- Daily budget: ~500–1,000 calls typical → well under 10,000/day limit
- Do NOT add features that loop all users every minute with multiple API calls each

## Docker Stack

Services: `dota-discord-bot`, `discord-dota-postgres` (PG 16), `discord-dota-minio`

- Image: `orgmcr.or-gm.com/osmargm1202/dota-discord-bot:latest`
- Build: `make build` (builds + pushes in one step)
- Deploy: `docker compose up -d` on server at `~/discord-dota/`
- Server: `aj@server.fifrex.com`
- MinIO public URL via CloudFlare tunnel: `https://dota-s3.fifrex.com`
- CloudFlare tunnel network: `cloudflared` (external Docker network, must exist on server)

## Key Environment Variables

| Variable            | Notes                                              |
|---------------------|----------------------------------------------------|
| `POSTGRES_DSN`      | Uses container name: `discord-dota-postgres:5432`  |
| `MINIO_ENDPOINT`    | Uses container name: `discord-dota-minio:9000`     |
| `MINIO_PUBLIC_URL`  | `https://dota-s3.fifrex.com` (CloudFlare tunnel)  |
| `RANKING_CHANNEL_ID`| `1519494823642398852`                              |
| `BASE_YEAR`         | Year from which to start historical backfill       |
| `BACKFILL_DELAY_MS` | `700` — keeps Stratz calls under rate limit        |

## Architecture Notes

- Match notifications: PNG generated with `fogleman/gg`, uploaded to MinIO `match-notifications/` prefix, URL sent as Discord embed + Stratz link
- Ranking channel: 3 pinned messages (individual, team2, team3), updated as Discord file attachments after each match
- Backfill: idempotent (skips already-stored matches), runs at startup + every 6h
- MinIO cleanup: daily goroutine deletes `match-notifications/` images older than 30 days
- `/dota ranking` slash command: sends PNG as file attachment (no MinIO needed)
