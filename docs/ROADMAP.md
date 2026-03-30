# hone — Full Implementation Roadmap

## Context

The hone project foundation is complete: Go module, Cobra command skeleton, Viper config, SQLite with goose migrations (full schema), and the SM-2 `UpdateEF` function with tests. This plan covers all remaining work to reach a fully functional CLI.

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

Quality mapping uses two thresholds per difficulty (`fast` and `normal`). Duration < `fast` → quality 5; < `normal` → quality 4; ≥ `normal` → quality 3.

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
- Completion is signaled via typed messages (e.g. `PushMsg`, `PopMsg`) so parent models can react without coupling.
- The same component runs standalone (wrapped by `tui.Run()`) from a Cobra command, or embedded inside the stats dashboard by routing messages to it.
- `tui.Run(m tea.Model)` is a one-liner wrapper: `tea.NewProgram(m, tea.WithAltScreen()).Run()`.
- Navigation uses a router stack (`internal/tui/router.go`): `PushMsg` pushes a new model, `Pop()` returns a command that pops back.

Package layout:

```
internal/tui/
  router.go         — router stack (Push/Pop navigation)
  run.go            — Run() helper
  keys.go           — all key.Map types + newHelpModel()
  color_table.go    — ANSI-safe custom table (replaces bubbles/table)
  dashboard.go      — tab bar + tab routing (Stats/Problems/Playlists/Topics)
  stats_tab.go      — stats tab with metric cards + per-topic/playlist progress
  problems_tab.go   — problems list with filter, sort, colorTable
  topics_tab.go     — topics list with sort modes, topic filter selection
  playlist.go       — playlist hub (list + create + problem picker) + select model
  playlist_picker.go — multi-select problem picker for adding to a playlist
  practice.go       — practice session (timer + browser result channel)
  add.go            — add-problem flow (textinput → spinner → card)
```

### Custom `colorTable` instead of `bubbles/table`

`bubbles/table` (v1.0.0) uses `runewidth.Truncate()` internally which does **not** understand ANSI escape sequences — it counts escape bytes as visible characters, causing premature truncation and column misalignment when cells contain lipgloss-styled text. `internal/tui/color_table.go` is a lightweight replacement that uses `lipgloss.Width()` (ANSI-aware) for all width calculations. Use `colorTable` for any table that needs styled cell content.

### PracticeFilter and mutual exclusivity

`store.PracticeFilter{PlaylistID *int, TopicID *int}` is the unified filter threaded through the app. Playlist and topic are mutually exclusive: `config.SetActivePlaylist` clears `active_topic_id` and vice versa. The filter is re-read from config on every tab switch so all views stay in sync.

---

## Phase 1: SRS Engine ✅ COMPLETE

Pure SM-2 logic + DB helpers. `QualityFromDuration`, `UpdateSRS`, `PickNext`, `RecordAttempt`, `SaveSRSState`. Full test coverage with in-memory SQLite.

---

## Phase 2: `hone add` ✅ COMPLETE

URL parsing (`internal/platform/`), headless Rod scraping (`internal/scraper/`), batch import via `-f` flag with progress bar TUI. Problems inserted with topics via upsert.

---

## Phase 3: `hone playlist` ✅ COMPLETE

Playlist CRUD in TUI (hub model with list + create form + problem picker). Active playlist stored in config. Mutually exclusive with topic filter.

---

## Phase 4: `hone practice` ✅ COMPLETE

Headful Rod browser monitor (`internal/monitor/`). Practice TUI with live timer, browser result via Go channel, SRS update + attempt record on completion.

---

## Phase 5: TUI Dashboard ✅ COMPLETE

Full tabbed dashboard (Stats / Problems / Playlists / Topics) with:

- Stats tab: metric cards, segmented progress bars, per-topic breakdown, "Currently practicing" section for active playlist or topic
- Problems tab: colorTable with filter, sort, difficulty/progress bars
- Playlists tab: list + create + problem picker; toggle-select (Enter on active playlist deselects it); mutual exclusivity with topic
- Topics tab: colorTable with 3 sort modes (alpha / % mastered / weakest first); Enter toggles topic as active filter; `*` marker only shown when topic is the active filter
- Global `p` key from any tab starts a practice session on the current filter
- `bubbles/help` key bar on every tab
- Tab bar shows active filter indicator; router stack for push/pop navigation

---

## Phase 6: Distribution

**Goal:** Package for Homebrew installation.

**Files:** `.goreleaser.yaml`, separate tap repo

### 6.1 GoReleaser config
- macOS arm64 + amd64 builds
- Homebrew tap configuration

### 6.2 Homebrew tap repo
- Create `homebrew-tap` repo
- GoReleaser auto-generates formula on release

### 6.3 CI/CD
- GitHub Actions workflow: test → build → release on tag

---

## Summary: implementation order

| Phase | Deliverable | Status |
|-------|------------|--------|
| 1 | SRS engine (pick, update, record) | ✅ complete |
| 2 | `hone add` (URL parse, scrape, insert) | ✅ complete |
| 3 | `hone playlist` (create, list, select) | ✅ complete |
| 4 | `hone practice` (browser + SRS) | ✅ complete |
| 5 | TUI dashboard (all tabs + navigation) | ✅ complete |
| 6 | Distribution (GoReleaser + Homebrew) | not started |
