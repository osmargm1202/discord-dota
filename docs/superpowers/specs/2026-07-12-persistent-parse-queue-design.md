# Persistent Stratz Parse Queue Design

## Problem

`CheckForNewMatches` only examines the newest Stratz match (`matches[0]`). While that match is unparsed, the bot retries it without advancing `lastMatchID`. If another match appears, the bot switches to the newer match and can abandon the older one.

The visible queue is an in-memory map keyed by Discord user, so it can hold only one match per user, does not replace stale entries, and disappears on restart. PostgreSQL has a `parse_queue` table, but detection never calls `EnqueueParse`; consequently `MarkParseDone` usually updates no row and there is no persistent queue consumer.

## Required Behavior

- Discover and retain every recent match not previously discovered.
- Never let one unparsed match block another match for the same player.
- Prefer newer pending matches when processing. A newer parsed match may be notified before an older unparsed match.
- Persist pending work across restarts.
- Avoid duplicate queue entries and duplicate notifications.
- Continue respecting Stratz API limits.

## Architecture

PostgreSQL becomes the source of truth for parse work. Queue identity includes both `match_id` and `dota_id`, because one match may contain multiple registered players and each player needs an independent notification and `player_matches` row.

The polling flow is split conceptually into two phases:

1. **Discovery** fetches up to five recent matches for each registered player, selects matches newer than that player's discovery checkpoint, and enqueues each idempotently. It then advances the checkpoint to the greatest discovered match ID.
2. **Processing** loads pending queue rows newest-first. Each row fetches its Stratz match independently. Unparsed rows request parsing and remain pending. Parsed rows generate the player notification, persist match/player statistics, and become done only after successful notification and persistence.

Processing newer rows first implements the selected policy: a stale unparsed row cannot block a newer parsed row.

## Data Model

Change `parse_queue` from match-only identity to per-player work:

- `match_id BIGINT`
- `dota_id BIGINT`
- `discord_id VARCHAR(20)`
- `enqueued_at`, `last_attempt`, `attempt_count`
- `status` (`pending`, `done`)
- composite primary key `(match_id, dota_id)`

Migration remains idempotent. Existing match-only pending rows cannot reliably identify recipients, so discovery repopulates actionable per-player rows from recent matches. Existing completed historical match data remains untouched.

Queue query returns structured rows rather than bare match IDs. `/dota queue` reads persistent pending rows, with optional in-memory enrichment unnecessary for correctness.

## State and Idempotency

`last_processed_match` is reinterpreted as the greatest discovered match ID for each Dota account. It advances after successful enqueueing of all selected recent matches, not after notification.

Enqueue uses `ON CONFLICT DO NOTHING`. A queue row becomes `done` only after notification succeeds and database persistence completes. A failed notification stays pending for retry. Match and player writes remain idempotent through existing upserts.

A single polling invocation must not overlap another invocation. Add a bot-level mutex around `CheckForNewMatches`; overlapping startup and ticker calls return or serialize, preventing duplicate Discord sends.

## API Budget

Discovery remains one recent-match request per user per polling interval. Processing makes one match-detail request per pending row per cycle. To prevent an old backlog from exhausting limits, process a bounded number of pending rows per cycle and request parse only according to retry timing recorded in `last_attempt`. Default behavior should remain comfortably below documented Stratz limits.

## Error Handling

- Recent-match fetch failure affects only that user and does not advance discovery checkpoint.
- Enqueue failure prevents checkpoint advancement for that user, allowing safe rediscovery.
- Match-detail failure increments attempt metadata and leaves row pending.
- Unparsed match requests parse, increments attempt metadata, and remains pending.
- Missing player data remains pending for bounded retries; after the existing retry threshold it is marked done with an explicit warning to avoid permanent poison rows.
- Notification or persistence failure leaves row pending.
- Queue/database errors are logged and do not prevent other users from discovery where safe.

## Tests

Use test-first development around isolated selection and queue-processing logic:

1. Discovery selects all matches newer than checkpoint, not only `matches[0]`.
2. Discovery enqueues in an idempotent manner and advances checkpoint only after all enqueues succeed.
3. Pending rows are ordered newest-first.
4. Older unparsed row does not block newer parsed row.
5. Unparsed row requests parse and remains pending.
6. Successful parsed row notifies, persists, and becomes done.
7. Notification failure leaves row pending.
8. Persistent rows remain available after constructing a new bot/process context.
9. Full Go test suite and race detector pass.

## Deployment

After tests and build verification:

1. Run `make build` to build and push `orgmcr.or-gm.com/osmargm1202/dota-discord-bot:latest`.
2. SSH to `aj@server.fifrex.com` (or alias `fifrex`).
3. In `~/discord-dota`, pull and recreate the bot service with Docker Compose.
4. Confirm service health/status and inspect bounded startup logs for migration, Discord connection, queue discovery, and absence of fatal errors.
