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

## Phase 6: `hone import` — Playlist-aware Bulk Import

**Goal:** Import problems from a file with optional playlist grouping. Unlike `add -f` (flat URL list), import understands playlist headers and handles duplicates gracefully.

**Files:** `cmd/import.go`, `internal/importer/importer.go`

### File format

```
# Favorites
https://neetcode.io/problems/two-sum/question
https://neetcode.io/problems/valid-anagram/question

# Week 1
https://neetcode.io/problems/eating-bananas/question

https://neetcode.io/problems/climbing-stairs/question
```

- Lines starting with `#` define a playlist boundary. All URLs below belong to that playlist until the next `#` line.
- If the playlist doesn't exist, create it. If it exists, use the existing one.
- If a URL's problem already exists in the DB, skip scraping and add the existing problem to the current playlist (if any). If no playlist context, do nothing for existing problems.
- If a problem doesn't exist, scrape and insert it, then add to the current playlist (if any).
- URLs outside any `#` section are imported as standalone problems (no playlist assignment).
- Blank lines and lines starting with `//` are ignored.

### 6.1 File parser (`internal/importer/importer.go`)
- `ParseImportFile(path string) ([]ImportGroup, error)` — returns ordered groups, each with an optional playlist name and a list of URLs
- `ImportGroup{Playlist string, URLs []string}` — empty Playlist means "no playlist"

### 6.2 Import engine
- `RunImport(db *sqlx.DB, profileDir string, groups []ImportGroup, progress func(ImportProgress))` — processes groups sequentially
- For each URL: check if problem exists (by platform+slug), skip scraping if so, create playlist if needed, add problem→playlist link
- `ImportProgress{Current, Total int, URL, Status string}` — callback for TUI feedback

### 6.3 Inline progress TUI (`cmd/import.go`)
- Runs **inline** (no altscreen) — prints progress line-by-line as each URL is processed
- Spinner on the current item, checkmark/skip/error per completed item
- Summary at the end: X added, Y skipped (existing), Z errors, N playlists created

### 6.4 Wire up `cmd/import.go`
- `hone import FILENAME` — single positional argument
- Validate file exists and is readable before starting

### 6.5 Tests
- Parser: table-driven tests for various file formats (playlists, mixed, empty lines, comments)
- Import engine: integration tests with in-memory DB (mock scraper or pre-seeded problems)

---

## Phase 7: `hone export` + `hone init` — Backup & Restore

**Goal:** Export problem/playlist data in a human-readable format and full database state as a JSON backup. Restore from backup with `hone init`.

**Files:** `cmd/export.go`, `cmd/init.go`, `internal/backup/backup.go`

### Export formats

**Default (`hone export`)** — human-readable playlist format, same as the import file format:
- Groups problems by playlist under `#PlaylistName` headers
- Problems not in any playlist appear at the top, before any `#` header
- Each problem is rendered as its URL (constructed from platform + slug + URL template)
- Round-trips with `hone import`

**Full backup (`hone export --backup`)** — JSON dump of the entire database state:
```json
{
  "version": 1,
  "exported_at": "2026-03-30T...",
  "problems": [
    {
      "platform": "neetcode", "slug": "two-sum", "title": "Two Sum",
      "difficulty": "easy", "topics": ["arrays", "hashing"],
      "srs": { "ef": 2.5, "interval": 6, "reps": 2, "next_review": "2026-04-05", "mastered_before": false },
      "attempts": [
        { "started_at": "...", "completed_at": "...", "result": "success", "duration_sec": 480, "quality": 5 }
      ]
    }
  ],
  "playlists": [
    { "name": "Favorites", "problems": ["neetcode/two-sum", "neetcode/valid-anagram"] }
  ],
  "config": {
    "active_playlist": "Favorites",
    "active_topic": null
  }
}
```

### 7.1 Export queries (`internal/backup/backup.go`)
- `ExportPlaylistFormat(db *sqlx.DB) (string, error)` — generates the `#Playlist` + URL text
- `ExportFullBackup(db *sqlx.DB) (BackupData, error)` — collects all problems, SRS state, attempts, playlists, config into a struct
- `BackupData` struct with `json` tags, versioned schema

### 7.2 `cmd/export.go`
- `hone export` — writes playlist format to stdout (pipe-friendly) or to a file with `-o FILENAME`
- `hone export --backup` — writes JSON to stdout or `-o FILENAME`
- `hone export --backup -o backup.json` — common usage

### 7.3 Restore (`internal/backup/restore.go`)
- `RestoreFromBackup(dbPath string, data BackupData) error` — creates a fresh DB at `dbPath`, applies migrations, inserts all data
- Validates backup version, rejects if DB already exists at the path

### 7.4 `cmd/init.go`
- `hone init BACKUPFILE` — reads the JSON backup file, creates the database, restores all state
- Fails with a clear error if `~/.local/share/hone/data.db` already exists ("database already exists, use `rm` to reset first")
- Prints summary: X problems, Y playlists, Z attempts restored

### 7.5 Tests
- Export: round-trip test — seed DB, export playlist format, parse with import parser, verify equivalence
- Backup/restore: seed DB, export JSON, restore into new in-memory DB, verify all data matches
- Init: verify it refuses to overwrite an existing DB

---

## Phase 8: Documentation

**Goal:** Add an mkdocs-based documentation site covering both end-user guides and developer/contributor documentation.

**Files:** `mkdocs.yml`, `docs/` (restructured)

### 8.1 mkdocs setup
- `mkdocs.yml` with Material for MkDocs theme
- GitHub Pages deployment via GitHub Actions (build on push to main)
- Navigation structure matching sections below

### 8.2 User documentation
- **Getting started** — installation (Homebrew, from source), first run, adding your first problem
- **Commands reference** — every command (`hone`, `hone add`, `hone practice`, `hone playlist`, `hone import`, `hone export`, `hone init`) with examples
- **Concepts** — spaced repetition overview, how the SM-2 algorithm works in hone, quality mapping, mastered_before boost
- **Dashboard guide** — tabs walkthrough (Stats, Problems, Playlists, Topics), keyboard shortcuts, filters
- **Import/export** — file format spec, backup/restore workflow, migration between machines
- **Configuration** — config file location, overridable thresholds, platform URL templates, browser profile

### 8.3 Developer documentation
- **Architecture overview** — package map (`srs/`, `store/`, `tui/`, `scraper/`, `monitor/`, `config/`, `platform/`), data flow diagrams
- **Adding a new platform** — URL parser, scraper selectors, monitor selectors, URL template config
- **TUI development** — router stack, embedded vs standalone models, `PushMsg`/`PopMsg`, `colorTable` usage
- **Testing** — `db.OpenMemory()`, table-driven tests, when to use integration vs unit tests
- **Contributing** — build instructions, PR workflow, code style

### 8.4 CI integration
- GitHub Actions step: `mkdocs build --strict` on every PR to catch broken links/syntax
- Deploy to GitHub Pages on merge to main

---

## Phase 9: Distribution

**Goal:** Package for Homebrew installation.

**Files:** `.goreleaser.yaml`, separate tap repo

### 9.1 GoReleaser config
- macOS arm64 + amd64 builds
- Homebrew tap configuration

### 9.2 Homebrew tap repo
- Create `homebrew-tap` repo
- GoReleaser auto-generates formula on release

### 9.3 CI/CD
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
| 6 | `hone import` (playlist-aware bulk import) | ✅ complete |
| 7 | `hone export` + `hone init` (backup/restore) | ✅ complete |
| 8 | Documentation (mkdocs site) | ✅ complete |
| 9 | Distribution (GoReleaser + Homebrew) | ✅ complete |
