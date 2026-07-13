# Persistent Stratz Parse Queue Implementation Plan

> **For agentic workers:** REQUIRED SUB-SKILL: Use superpowers:subagent-driven-development (recommended) or superpowers:executing-plans to implement this plan task-by-task. Steps use checkbox (`- [ ]`) syntax for tracking.

**Goal:** Persist every newly discovered player match and process newer parsed matches without being blocked by older unparsed matches.

**Architecture:** PostgreSQL stores per-player queue rows keyed by `(match_id, dota_id)`. Polling separates discovery from processing: discovery enqueues all recent IDs above a checkpoint, while processing independently handles bounded pending rows newest-first.

**Tech Stack:** Go, PostgreSQL 16, `database/sql`, DiscordGo, Stratz GraphQL client, Docker Compose.

## Global Constraints

- Keep Stratz usage below 20 requests/second, 250/minute, 2,000/hour, and 10,000/day.
- Process newer pending rows before older rows.
- One unparsed row must not block another row.
- Queue state must survive restart.
- Queue writes and notifications must be idempotent where possible.
- Use test-first development and run race detection before deployment.

---

### Task 1: Pure discovery selection

**Files:**
- Create: `discord/match_queue.go`
- Create: `discord/match_queue_test.go`

**Interfaces:**
- Consumes: `[]dota.StratzMatch` returned by `GetPlayerRecentMatches`.
- Produces: `selectUndiscoveredMatchIDs(matches, checkpoint, hasCheckpoint) []int64`.

- [ ] **Step 1: Write failing tests** proving all IDs above checkpoint are selected, newest-first order is preserved, and no-checkpoint discovery seeds checkpoint without replaying old history according to current startup semantics.
- [ ] **Step 2: Run** `go test ./discord -run TestSelectUndiscoveredMatchIDs -v`; expect compile failure because helper is undefined.
- [ ] **Step 3: Implement minimal pure helper** in `discord/match_queue.go`, filtering every recent result rather than only index zero.
- [ ] **Step 4: Run** `go test ./discord -run TestSelectUndiscoveredMatchIDs -v`; expect PASS.
- [ ] **Step 5: Commit** `git add discord/match_queue.go discord/match_queue_test.go && git commit -m "test: define multi-match discovery semantics"`.

### Task 2: Persistent per-player queue repository

**Files:**
- Modify: `internal/db/schema.sql`
- Modify: `internal/db/queries.go`
- Create: `internal/db/queue_test.go`

**Interfaces:**
- Produces:
  - `type ParseQueueRow struct { MatchID int64; DotaID int64; DiscordID string; EnqueuedAt time.Time; LastAttempt sql.NullTime; AttemptCount int }`
  - `EnqueueParse(matchID, dotaID int64, discordID string) error`
  - `GetPendingParseQueue(limit int) ([]ParseQueueRow, error)` ordered by `match_id DESC`
  - `MarkParseDone(matchID, dotaID int64) error`
  - `IncrementParseAttempt(matchID, dotaID int64) error`

- [ ] **Step 1: Write failing repository tests** using the project's PostgreSQL test strategy, covering composite identity, two rows for one player, newest-first order, idempotent enqueue, attempt increment, and per-player completion.
- [ ] **Step 2: Run** `go test ./internal/db -run TestParseQueue -v`; expect failures from old signatures/schema.
- [ ] **Step 3: Add idempotent migration** that adds `dota_id` and `discord_id`, removes unusable legacy rows missing recipient identity, replaces match-only primary key with `(match_id, dota_id)`, and adds pending-order index.
- [ ] **Step 4: Replace queue query methods** with structured per-player versions and bounded pending query.
- [ ] **Step 5: Run** `go test ./internal/db -run TestParseQueue -v`; expect PASS (or explicit integration-test skip when test DSN is absent).
- [ ] **Step 6: Commit** `git add internal/db/schema.sql internal/db/queries.go internal/db/queue_test.go && git commit -m "feat: persist per-player parse queue"`.

### Task 3: Split discovery and independent queue processing

**Files:**
- Modify: `discord/bot.go`
- Modify: `discord/match_queue.go`
- Modify: `discord/match_queue_test.go`

**Interfaces:**
- `CheckForNewMatches() error` serializes invocations with a mutex.
- Discovery calls repository queue methods for every selected recent match before advancing checkpoint.
- Processing consumes at most a fixed safe batch, newest-first.

- [ ] **Step 1: Add failing tests** with narrow fake interfaces for recent-match discovery and pending-row processing. Verify two new matches enqueue, enqueue failure does not advance checkpoint, older unparsed row requests parse and remains pending, and newer parsed row completes despite older pending row.
- [ ] **Step 2: Run** `go test ./discord -run 'TestDiscover|TestProcessParseQueue' -v`; expect failure because orchestration helpers do not exist.
- [ ] **Step 3: Extract minimal interfaces/helpers** so tests invoke real orchestration without Discord/network calls. Keep production adapters on existing `StratzClient`, `UserStore`, and `DB`.
- [ ] **Step 4: Change discovery** to scan all five recent matches, enqueue each selected `(match_id,dota_id,discord_id)`, and advance checkpoint only after all enqueues succeed. For a player with no checkpoint, preserve current no-replay startup behavior by setting checkpoint to newest without notifying historical matches.
- [ ] **Step 5: Change processing** to load bounded pending rows newest-first; fetch each independently; request parse and increment attempt for unparsed rows; notify/persist/mark done for parsed rows; continue after row-local failure.
- [ ] **Step 6: Remove correctness dependency on `pendingMatches`** and change `/dota queue` to PostgreSQL rows. Remove shared map or protect any remaining display cache with mutex.
- [ ] **Step 7: Run** `go test ./discord -run 'TestDiscover|TestProcessParseQueue|TestSelectUndiscoveredMatchIDs' -v`; expect PASS.
- [ ] **Step 8: Run** `go test -race ./discord ./internal/db`; expect PASS or documented DB integration skips only.
- [ ] **Step 9: Commit** `git add discord/bot.go discord/match_queue.go discord/match_queue_test.go && git commit -m "fix: process persistent match queue independently"`.

### Task 4: Full verification and compatibility

**Files:**
- Modify only files required by test/build failures.

**Interfaces:**
- Existing slash commands and notification rendering remain compatible.

- [ ] **Step 1: Run** `gofmt -w discord/match_queue.go discord/match_queue_test.go discord/bot.go internal/db/queries.go internal/db/queue_test.go`.
- [ ] **Step 2: Run** `go test ./...`; expect all packages PASS.
- [ ] **Step 3: Run** `go test -race ./...`; expect all packages PASS.
- [ ] **Step 4: Run** `go vet ./...`; expect no diagnostics.
- [ ] **Step 5: Run** `go build ./...`; expect exit 0.
- [ ] **Step 6: Inspect** `git diff --check` and focused diff; expect no whitespace errors or unrelated changes.
- [ ] **Step 7: Commit any compatibility corrections** with a focused message, then leave clean tree.

### Task 5: Build, publish, and production deployment

**Files:**
- No source changes expected.

**Interfaces:**
- Image: `orgmcr.or-gm.com/osmargm1202/dota-discord-bot:latest`.
- Server: `aj@server.fifrex.com`.
- Deployment directory: `~/discord-dota`.

- [ ] **Step 1: Run** `make build`; expect successful image build and registry push.
- [ ] **Step 2: Push source commits** with `git push origin main`; expect remote update success.
- [ ] **Step 3: SSH and inspect deployment directory** using `ssh aj@server.fifrex.com 'cd ~/discord-dota && docker compose config --services'`; expect `dota-discord-bot`, `discord-dota-postgres`, and `discord-dota-minio`.
- [ ] **Step 4: Pull and recreate bot** using `ssh aj@server.fifrex.com 'cd ~/discord-dota && docker compose pull dota-discord-bot && docker compose up -d dota-discord-bot'`; expect successful pull and running container.
- [ ] **Step 5: Verify status** using `ssh aj@server.fifrex.com 'cd ~/discord-dota && docker compose ps'`; expect bot and dependencies running.
- [ ] **Step 6: Inspect bounded logs** using context-mode around SSH command for the last 200 bot lines. Confirm migrations applied, Discord connected, no fatal/panic, and polling starts.
- [ ] **Step 7: Report deployed commit, image, container state, and any queue migration observations.**
