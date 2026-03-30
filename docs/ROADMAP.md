# hone — Full Implementation Roadmap

## Context

The hone project foundation is complete: Go module, Cobra command skeleton (all stubs), Viper config, SQLite with goose migrations (full schema), and the SM-2 `UpdateEF` function with tests. This plan covers all remaining work to reach a fully functional CLI.

Phases are ordered by dependency — each builds on the previous. Within a phase, steps can often be parallelized. Each phase ends with a working, testable increment.

---

## Architectural Decisions

### `internal/srs/` vs `internal/store/` separation

`internal/srs/` contains only pure functions (SM-2 math, quality mapping, state transitions) with no dependencies on the database or config. `internal/store/` contains all DB query functions (`PickNext`, `RecordAttempt`, `SaveSRSState`) and imports `srs` for types.

The wiring: Cobra commands / Bubble Tea models call `store` to fetch data, pass it to `srs` for computation, then call `store` again to persist. If the DB layer changes, only `store` changes. If the algorithm changes, only `srs` changes.

### Testing conventions

- SM-2 pure functions (`UpdateEF`, `QualityFromDuration`, `UpdateSRS`) must have comprehensive table-driven tests covering all edge cases — no DB setup required.
- Store and picker tests use `db.OpenMemory()` (an in-memory SQLite helper in the `db` package that applies all migrations) so tests are isolated and self-contained.

### Config thresholds

Quality mapping uses two thresholds per difficulty (`fast` and `normal`). Duration < `fast` → quality 5; < `normal` → quality 4; ≥ `normal` → quality 3. The `slow` key is not used.

### mastered_before boost

The interval boost for previously-mastered problems applies when `mastered_before == true` AND `repetition_count > 1` (after incrementing on a success). This means the user has had at least two consecutive successes since coming back — clearly still knows it.

### TUI layering

Three tiers, chosen by command complexity:

1. **Lipgloss-styled output** — commands that run and exit (e.g. `playlist list`, `playlist create`). No Bubble Tea program; just `lipgloss.NewStyle()` on printed output.
2. **Spinner + result card** — commands with async work or optional input (e.g. `hone add`). A minimal Bubble Tea program: textinput → spinner while scraping/saving → lipgloss result card, then any-key-to-exit.
3. **Full Bubble Tea program** — views with continuous state (e.g. `hone practice` with a live timer, `hone stats` as the navigation hub).

### TUI component architecture

Components live in `internal/tui/` and follow the embedded-model pattern:

- Each component is a `tea.Model` with its own `Init/Update/View`.
- Completion is signaled via typed messages (e.g. `PlaylistSelectedMsg`, `ProblemAddedMsg`) so parent models can react without coupling.
- The same component runs standalone (wrapped by `tui.Run()`) from a Cobra command, or embedded inside the stats dashboard by routing messages to it.
- `tui.Run(m tea.Model)` is a one-liner wrapper: `tea.NewProgram(m, tea.WithAltScreen()).Run()`.

Package layout:

```
internal/tui/
  run.go                    — Run() helper
  add.go                    — add-problem flow (textinput → spinner → card)
  playlist.go               — playlist flow (select list + create form)
  stats.go                  — stats dashboard hub (embeds other components)
  practice.go               — practice session (timer + browser result channel)
```

Save `huh` for multi-field forms (e.g. playlist creation with extra fields). Single-field inputs use `bubbles/textinput` directly.

---

## Phase 1: SRS Engine (core logic, no UI)

**Goal:** Complete the spaced repetition engine so it can pick problems and update state after attempts. Pure logic + database queries, fully testable without TUI or browser.

**Files:** `internal/srs/srs.go`, `internal/srs/srs_test.go`, `internal/store/store.go`, `internal/store/store_test.go`

### 1.1 Quality mapping
- `QualityFromDuration(durationMin int, t Thresholds) int` — returns 3/4/5 based on passed thresholds (caller looks up difficulty-specific thresholds from config)
- Failure quality is handled by the caller (hardcoded to 1), not this function

### 1.2 SRS state update
- `UpdateSRS(state ProblemSRS, result string, durationMin int, difficulty string) ProblemSRS` — applies the full algorithm:
  - On success: increment rep count, compute interval (rep 1→1, rep 2→6, rep 3+→interval×EF), mastered_before boost, update EF
  - On failure: reset rep count to 0, interval to 1, update EF with quality=1, clamp EF≥1.3
  - Set next_review_date = today + interval_days
  - If interval > 60 days, set mastered_before = true
- Uses existing `UpdateEF`

### 1.3 Picker queries
- `store.PickNext(db *sqlx.DB, playlistID *int) (*Problem, *srs.ProblemSRS, bool, error)` — returns problem + SRS state + whether it's due or upcoming
  - Filter by active playlist (if set)
  - Due: `next_review_date <= today`, pick most overdue
  - If nothing due: pick nearest future review date
  - Return bool indicating due vs. upcoming (for TUI messaging)

### 1.4 Database helpers (in `internal/store/`)
- `RecordAttempt(db, problemID, startedAt, completedAt, result, durationSec, quality)` — insert attempt row
- `SaveSRSState(db, srs.ProblemSRS)` — update problem_srs row

### 1.5 Tests
- Table-driven tests for `QualityFromDuration` (all difficulty×duration combos + boundaries)
- Tests for `UpdateSRS` covering: first success, second success, third+ success, failure, failure after mastery, mastered_before boost
- Picker tests using an in-memory SQLite DB with seeded data

---

## Phase 2: `hone add` — Problem Ingestion

**Goal:** Parse a URL, scrape problem metadata with Rod, and insert into the database.

**Files:** `internal/platform/platform.go`, `internal/scraper/scraper.go`, `cmd/add.go`

### 2.1 URL parser (`internal/platform/platform.go`)
- `ParseURL(rawURL string) (platform, slug string, err error)`
- Recognize LeetCode and NeetCode URL patterns, extract slug
- Return error for unrecognized URLs
- `BuildURL(platform, slug string) string` — construct URL from platform templates in Viper config

### 2.2 Scraper (`internal/scraper/scraper.go`)
- `Scrape(platform, slug string) (title, difficulty string, topics []string, err error)`
- Headless Rod: launch browser, navigate to problem URL, extract metadata from DOM
- Platform-specific selectors for LeetCode and NeetCode
- Timeout handling

### 2.3 Wire up `cmd/add.go`
- Accept URL as argument or prompt with huh form
- Parse URL → scrape → insert problem + topics (upsert topics, link via problem_topics)
- Print confirmation with problem details

### 2.4 Tests
- URL parser: table-driven tests with various URL formats
- Scraper: skip in CI (needs browser), manual verification instructions in test comments

---

## Phase 3: `hone playlist` — Playlist Management

**Goal:** Create, list, and select playlists. Store active selection in config.

**Files:** `cmd/playlist.go`, `internal/config/config.go`

### 3.1 `playlist create`
- Accept name as argument or prompt
- Insert into playlists table
- Optionally add problems to it (future: interactive picker)

### 3.2 `playlist list`
- Query all playlists with problem counts
- Print as formatted table

### 3.3 `playlist select`
- Accept name or ID
- Write active playlist to config file via `viper.Set` + `viper.WriteConfig`
- Add `config.ActivePlaylistID() *int` accessor

### 3.4 Tests
- Integration tests with in-memory DB for create/list
- Config write/read round-trip for select

---

## Phase 4: `hone practice` — Practice Sessions

**Goal:** Pick a problem, open it in the browser, monitor for completion, record the attempt.

**Files:** `internal/monitor/monitor.go`, `cmd/practice.go`

### 4.1 Browser monitor (`internal/monitor/monitor.go`)
- `Monitor(ctx context.Context, platform, slug string) <-chan Result` — returns channel
- Headful Rod: open problem URL, poll DOM for submission result indicators
- Platform-specific selectors for success/failure detection (LeetCode, NeetCode)
- Send `Result{Success bool, Timestamp time.Time}` on channel when detected
- Respect context cancellation

### 4.2 Practice flow in `cmd/practice.go`
- Call picker to get next problem
- Display problem info (title, difficulty, topics, due/upcoming status)
- Launch Rod browser via monitor
- Record start time
- Wait for result from channel (or user quit)
- Compute duration, derive quality, call `UpdateSRS`, `RecordAttempt`
- Print summary (result, duration, next review date)

### 4.3 Tests
- Mock-based test for the practice flow (mock picker, mock monitor channel)
- Monitor: manual verification (needs browser)

---

## Phase 5: TUI — Per-command polish + Stats Dashboard

**Goal:** Give each command a proper TUI (spinner, styled output, interactive input). Build the stats dashboard as the navigation hub that reuses these components inline.

**Files:** `internal/tui/` (new package), modifications to all `cmd/*.go`

### 5.1 `hone add` TUI (`internal/tui/add.go`)
- State machine: `stateInput` → `stateScraping` → `stateDone` / `stateErr`
- `stateInput`: `bubbles/textinput` for URL (skipped if URL provided as CLI arg)
- `stateScraping`: `bubbles/spinner` + async `tea.Cmd` running scrape + DB insert
- `stateDone`: lipgloss rounded-border card (title, difficulty colored, topics, platform)
- `stateErr`: lipgloss error card
- Any key exits from done/error states

### 5.2 `hone playlist` TUI (`internal/tui/playlist.go`)
- `PlaylistSelectModel`: `bubbles/list` of playlists; active one marked with `*`
- Emits `PlaylistSelectedMsg` / `PlaylistSelectCanceledMsg` for parent to handle
- Runs standalone from `hone playlist select`; embeddable in stats dashboard
- `hone playlist list` and `hone playlist create` use lipgloss-styled plain output (no BT program needed)

### 5.3 Stats dashboard (`internal/tui/stats.go`)
- Hub model with `viewState` enum: `viewStats`, `viewAdd`, `viewSelectPlaylist`, `viewPractice`
- Displays: total problems, due today, mastered count, per-topic table, recent attempts
- Keybindings: `p` practice, `a` add, `l` switch playlist, `q` quit
- Embeds `AddModel` and `PlaylistSelectModel`; routes focus and messages between them
- `cmd/root.go` RunE launches this

### 5.4 Practice view (`internal/tui/practice.go`)
- Problem info display + live timer (`tea.Tick` every second)
- Browser result arrives via channel wrapped in `tea.Cmd`
- Post-result summary before returning to stats view

### 5.5 Wire commands
- `cmd/root.go` RunE → launch stats dashboard
- `cmd/add.go` → `tui.Run(tui.NewAddModel(...))`
- `cmd/practice.go` → launch TUI at practice view
- `cmd/playlist.go select` → `tui.Run(tui.NewPlaylistSelectModel(...))`

---

## Phase 6: `hone topics` — Topic Filtering

**Goal:** List topics, show problems per topic, filter stats by topic.

**Files:** `cmd/topics.go`, queries in `internal/srs/` or `internal/db/`

### 6.1 Topic queries
- List all topics with problem counts and due counts
- List problems for a given topic with SRS state

### 6.2 Wire into TUI topics view (from Phase 5.6)

---

## Phase 7: Distribution

**Goal:** Package for Homebrew installation.

**Files:** `.goreleaser.yaml`, separate tap repo

### 7.1 GoReleaser config
- macOS arm64 + amd64 builds
- Homebrew tap configuration

### 7.2 Homebrew tap repo
- Create `homebrew-tap` repo
- GoReleaser auto-generates formula on release

### 7.3 CI/CD
- GitHub Actions workflow: test → build → release on tag

---

## Summary: implementation order

| Phase | Deliverable | Depends on |
|-------|------------|------------|
| 1 | SRS engine (pick, update, record) | Foundation ✅ |
| 2 | `hone add` (URL parse, scrape, insert) | Foundation ✅ |
| 3 | `hone playlist` (create, list, select) | Foundation ✅ |
| 4 | `hone practice` (browser + SRS) | Phase 1, Phase 2 |
| 5 | TUI (stats dashboard + navigation) | Phases 1–4 |
| 6 | `hone topics` (filtering + display) | Phase 5 |
| 7 | Distribution (GoReleaser + Homebrew) | Phase 5 |

Phases 1, 2, and 3 are independent and can be built in any order or in parallel.
