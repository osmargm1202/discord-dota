# /dota mystats — ability build win-rate design

## Purpose

Add `/dota mystats hero:<texto> level:<1-9> jugador:<@opcional>` — shows a PNG
card (same visual family as `/dota stats`) grouping a player's matches with a
given hero by the exact skill-point allocation (Q/W/E/R) at a chosen hero
level, with win/loss/draw record per group.

Example: Viper at level 6, group `1-1-3-1` (Q1-W1-E3-R1) → shows how many
times that build was played and its win rate, alongside other builds used
(e.g. `1-3-1-1`).

## Verified facts (2026-07-06 research session)

- Stratz `match(id).players.abilities` (and the same field nested inside
  `player.matches(...).players.abilities`) returns every skill-point event:
  `{abilityId, level, time, isTalent, abilityType{name}}`.
- `level` on an ability event is the ability's own rank (0-3), NOT hero level.
  Hero level = chronological index of picks, valid 1:1 **only for levels 1-9**
  (before first talent at level 10, which consumes a pick without being a
  Q/W/E/R skill point).
- Confirmed on real data: match `8883621088`, Fifrex (steamAccountId
  `208925877`), heroId 47 (Viper) → first 6 chronological picks =
  Q,E,E,W,E,R → tuple `1-1-3-1`, ultimate always the 6th pick in this sample.
- Batch query `player(steamAccountId).matches(request:{heroIds:[X],
  startDateTime:$after, take, skip}).players(steamAccountId).abilities{...}`
  returns abilities for N matches in one call — no per-match round-trip
  needed. Confirmed with 3 Viper matches in one request.
- No abilityId→slot(Q/W/E/R) mapping exists anywhere in this repo. Stratz's
  `abilityType.name` gives the ability's internal name (e.g.
  `viper_poison_attack`) but not its slot number.
- OpenDota's `GET /api/constants/hero_abilities` gives, per hero, an
  `abilities` array. **Index 0 = Q, index 1 = W, index 2 = E always**
  (confirmed on Viper, Axe, Juggernaut). The array's LAST index is **not**
  reliably the ultimate: Axe's array is `[axe_berserkers_call,
  axe_battle_hunger, axe_counter_helix, generic_hidden, generic_hidden,
  axe_culling_blade, axe_one_man_army]` — the real ultimate
  (`axe_culling_blade`) sits at index 5, followed by an Aghanim's Shard
  alt-cast ability (`axe_one_man_army`) at index 6. A naive "last index"
  rule would mislabel the Shard ability as the ultimate.
- Stratz's `AbilityStatType.isUltimate` field is **not reliable either** —
  confirmed on real data: querying it for Viper's actual ultimate
  (`viper_viper_strike`, abilityId 5221) returned `isUltimate: false`.
  Do not use this field.
- **Actual rule used (elimination, no ultimate-flag needed):** vendor only
  `abilities[0..2]` (Q/W/E names) per hero from `hero_abilities.json`. For
  each match, after collecting the distinct non-talent ability names in
  the player's pick log: any name matching `abilities[0]`→Q,
  `abilities[1]`→W, `abilities[2]`→E; **any other non-talent name
  encountered is R by elimination**. This works because
  `generic_hidden`/Shard-alt/Aghanim's-Scepter-granted abilities (e.g.
  `axe_one_man_army`, Viper's `viper_nose_dive`/`viper_predator`) are never
  separately skill-pointed by the player — they never appear as their own
  entry in the real Stratz `abilities` pick log — so the only 4th distinct
  name that can show up is the true ultimate. Confirmed against real data:
  Viper match `8883621088`'s pick log only ever contains
  `viper_poison_attack`, `viper_nethertoxin`, `viper_corrosive_skin`,
  `viper_viper_strike`, plus 2 talents — never `viper_nose_dive` or
  `viper_predator`.
- `talents` in the OpenDota payload is a separate array (`{name, level}`),
  redundant with Stratz's own `isTalent` flag — not used.
- No Discord autocomplete interaction handling exists in `discord/bot.go`
  (`InteractionApplicationCommandAutocomplete` is not handled anywhere).
- `dota/heroes.json` (vendored OpenDota dump) has `localized_name` but no
  hero→abilities-order mapping.

## Decisions

| Question | Decision |
|---|---|
| Hero input | Free text + fuzzy match against `heroes.json.localized_name` (no Discord autocomplete — not worth building the interaction handler for this) |
| Ability→slot mapping | Vendor OpenDota's `hero_abilities.json` (same pattern as existing `heroes.json`), mapping ability internal name → Q/W/E/R/talent slot |
| Level range | 1-9 only (fixed Discord choices). Levels ≥10 involve talents and are out of scope for now |
| Draw column | Always shown, always 0 (Dota has no draws; kept for parity with requested format) |
| Game modes included | All modes (ranked, unranked, turbo, etc.) — same scope as existing backfill (BaseYear 2026, no mode filter) |
| Command shape | Single command `/dota mystats` with 3 options: `hero` (required), `level` (required, choices 1-9), `jugador` (optional Discord user mention) |
| `jugador` resolution | If provided, resolve via `userStore` same as `handleMatchSlash`'s `account_id` option; if omitted, use invoking user's own registered account |
| Group sort order | By match count descending (most-played build first), not by win rate |

## Architecture

### Command registration (`discord/bot.go`)

New subcommand under the existing `dota` command (`registerCommands`,
~line 125-246):

```
mystats
  hero    STRING  required
  level   INTEGER required  choices: 1..9
  jugador USER    optional
```

New handler `handleMyStatsSlash`, following the shape of
`handleStatsSlash` (bot.go:809) and `handleMatchSlash` (bot.go:703) for
account resolution.

### Hero fuzzy match (`dota/api.go`)

New function, e.g. `FindHeroByName(query string) (heroID int, name string,
candidates []string, err error)`:
- Normalize (lowercase, strip accents/spaces) both query and each
  `localized_name`.
- Exact normalized match wins immediately.
- Otherwise substring match; if exactly one candidate, use it; if 2+,
  return them as `candidates` for an ambiguity error message; if 0, return
  "no match" error.

### Stratz query (`dota/stratz.go`)

New method `GetPlayerHeroAbilityBuilds(steamAccountID int64, heroID int,
afterUnix int64) ([]AbilityBuildMatch, error)`:
- Paginates `matches(request:{heroIds:[$heroId], startDateTime:$after,
  take, skip})` (same pagination shape as `GetPlayerMatchesForBackfill`,
  stratz.go:910) until a page returns fewer than `take` results.
- Requests per match: `id`, `didRadiantWin`,
  `players(steamAccountId:$steamAccountId){isRadiant abilities{abilityId
  time isTalent abilityType{name}}}`.

### Ability slot mapping (`dota/hero_abilities.json` + loader)

- Vendor `GET https://api.opendota.com/api/constants/hero_abilities` as
  `dota/hero_abilities.json`, same loading pattern as `loadHeroesLocal`
  (api.go:466). Only `abilities[0..2]` (Q/W/E) per hero are used — no
  attempt is made to identify R from this file (see rule above).
- Loader builds `map[string][3]string` (hero internal name → `[Q-name,
  W-name, E-name]`), i.e. just the first 3 entries of each hero's
  `abilities` array.

### Build computation (new file, e.g. `dota/ability_build.go`)

For each match returned by `GetPlayerHeroAbilityBuilds`:
1. Sort `abilities` by `time` ascending.
2. Take the first `level` entries. If fewer than `level` entries exist
   (remake/abandon before reaching that level), **skip this match**.
3. For each of the (at most 3) distinct non-talent ability names in that
   slice: if it matches the hero's `[Q-name, W-name, E-name]` vendor
   entry, bucket it there; otherwise bucket it as R (elimination rule — see
   above). If a 5th distinct non-talent name shows up (would mean the
   elimination assumption broke for this hero), **skip this match** and
   log a warning (don't crash).
4. Count picks per slot → tuple `(q, w, e, r)`.
5. Determine win: `didRadiantWin == isRadiant` for the target player.

Group matches by tuple. Per group: `wins`, `losses`, `draws` (always 0),
`total`, `winPct`. Sort groups by `total` descending.

### Rendering (`internal/ranking/image_mystats.go`, new file)

New `MyHeroStatsRenderData` struct: `PlayerName`, `AvatarBytes`, `HeroName`,
`HeroImageBytes`, `Level int`, `Groups []BuildGroupRow{Label string, Wins,
Losses, Draws, Total int}`, `TotalGames int`.

New `RenderMyHeroStats(d MyHeroStatsRenderData) ([]byte, error)`:
- Header: reuse `decodeImage`/`scaleImage` pattern from
  `drawStatsHeader` (image_stats.go:108) for the player avatar circle; add a
  square hero portrait next to it, fetched via `GetHeroImageURL` +
  MinIO cache (same fetch-and-cache pattern already used for avatars in
  `handleStatsSlash`, bot.go:855-863).
- Body: one row per build group — `Label` (e.g. `1-1-3-1`), G/W/L/E, %,
  in the same table style as `drawStatsGroup` (image_stats.go:162).
- Footer: total games analyzed, hero name, level, "Stratz" attribution —
  same style as `drawFooter`/existing footers.

## Error handling

| Case | Behavior |
|---|---|
| Hero: no match | Followup message: "No encontré el héroe '<x>'." |
| Hero: ambiguous | Followup message listing candidate names, ask to be more specific |
| Jugador (self or mentioned) not registered | Same message pattern as existing commands ("no está registrado, usa /dota register") |
| No matches found for hero+player since BaseYear | Followup message, same tone as `/dota stats`'s empty case |
| Match has no/incomplete `abilities` data (unparsed, remake, abandon) | Silently excluded from grouping, not shown as an error |
| 5th distinct non-talent ability name appears in the first N picks (elimination rule assumption broken for this hero) | That match excluded, `getLogger().Warnf` logged, command still completes |

## Testing

- Unit test: `FindHeroByName` — exact match, substring match, ambiguous
  (2+ candidates), no match.
- Unit test: build-tuple computation using the real fixture already
  captured in this session (Viper match 8883621088, first 6 chronological
  picks) — must compute `(1,1,3,1)` for level 6.
- Unit test: grouping/aggregation — given synthetic per-match results,
  correct wins/losses/% per group and correct sort order (by total desc).
- Manual verification: run `/dota mystats hero:Viper level:6` in Discord
  against Fifrex's real account, compare card output against the manual
  calculation already validated in this session.

## Out of scope (explicitly deferred)

- Levels ≥10 (talent-aware level counting)
- Discord native autocomplete for the `hero` option
- Persisting computed builds to the local database (this reads live from
  Stratz on each command invocation)
