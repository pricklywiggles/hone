# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Development Commands

```bash
go build ./...          # Build all packages
go test ./...           # Run all tests
go test ./internal/srs  # Run a single package's tests
go vet ./...            # Static analysis
go run .                # Run the CLI (creates DB on first run)
go run . --help         # Show command tree
```

Data: `~/.local/share/hone/data.db` | Config: `~/.config/hone/config.yaml`

To reset the database: `rm ~/.local/share/hone/data.db` (recreated on next run).

## Dependencies

- **modernc.org/sqlite** (no CGO) + **sqlx** — database
- **goose/v3** with `//go:embed` — migrations bundled in binary
- **cobra** / **viper** — CLI and config
- **bubbletea** / **bubbles** / **lipgloss** / **huh** — TUI
- **go-rod/rod** — browser automation (headful for practice, headless for scraping)

---

# System Prompt: hone — Coding Practice CLI

You are building a Go CLI application called hone for macOS that helps users practice coding problems using spaced repetition.

## Tech Stack

- **Cobra** — command structure
- **Viper** — configuration, XDG-compliant paths
- **Bubble Tea** with **bubbles/lipgloss/huh** — TUI
- **SQLite** via `modernc.org/sqlite` + `sqlx` — data storage (single file, no CGO)
- **Rod** (`go-rod/rod`) — browser automation (headful for practice, headless for scraping)
- **goose** or embedded SQL — schema migrations
- **Homebrew** — distribution via a Homebrew tap

## Architecture Principles

- Commands are defined with Cobra. TUI-driven commands launch a Bubble Tea program.
- Long-running external work (browser monitoring via Rod, database queries) communicates with Bubble Tea models through Go channels wrapped in `tea.Cmd` functions.
- Configuration lives in `~/.config/hone/config.yaml` managed by Viper. This includes the active playlist selection, duration thresholds, and platform URL templates (e.g. `platforms.neetcode.url_template: "https://neetcode.io/problems/{{slug}}/question"`). Problem URLs are constructed at runtime from platform + slug + template.
- Data lives in `~/.local/share/hone/data.db` as a SQLite database.
- The application is distributed as a Homebrew tap (`brew install [org]/tap/hone`). The build uses GoReleaser to produce macOS binaries and generate the Homebrew formula automatically.

## Default Behavior

Running the CLI with no arguments opens the **statistics dashboard** as the landing page. This is the home screen of the application. From this TUI, the user can navigate to all other functionality: starting a practice session, adding problems, managing playlists, and filtering by topic. Subcommands (e.g. `hone add`, `hone practice`) provide direct access to specific features for users who prefer it.

## Data Model

- **Problems** — id, platform (e.g. "leetcode", "neetcode", "geeksforgeeks"), slug (e.g. "eating-bananas"), title, difficulty, created_at
- **Topics** — id, name (e.g. "binary search", "dynamic programming")
- **ProblemTopics** — problem_id, topic_id (many-to-many)
- **Playlists** — id, name, created_at
- **PlaylistProblems** — playlist_id, problem_id, position (many-to-many, ordered)
- **Attempts** — id, problem_id, started_at, completed_at, result (success/fail), duration_seconds, quality (int, 1–5)
- **ProblemSRS** — problem_id, easiness_factor (float, default 2.5), interval_days (int, default 1), repetition_count (int, consecutive successes), next_review_date (date), mastered_before (bool, default false)

An active playlist OR active topic is stored in config (`active_playlist_id` / `active_topic_id`). The two are **mutually exclusive**: `config.SetActivePlaylist` clears `active_topic_id` and vice versa. The unified `store.PracticeFilter{PlaylistID *int, TopicID *int}` struct is threaded through the app; `store.ListPickQueue` and stats queries apply whichever field is non-nil.

## Picker Algorithm (SM-2 Based Spaced Repetition)

The picker uses a modified SM-2 algorithm adapted for coding problems.

### Picking the next problem

1. **Filter** to the active playlist. If no playlist is selected, all problems are candidates.
2. **Select due problems**: all candidates where `next_review_date <= today`.
3. If due problems exist, pick the most overdue (oldest `next_review_date`). Ties are broken by difficulty (easy → medium → hard), then by playlist position (if a playlist is active) or random selection.
4. If nothing is due, pick the problem with the nearest future `next_review_date` (closest to due). The TUI should communicate this distinction — e.g. "nothing due today, but here's one coming up" vs "you have 12 problems due."

### Updating SRS state after an attempt

Quality is determined automatically from the outcome and solve duration. No self-reporting.

**Quality mapping on success** — duration is compared against thresholds based on problem difficulty:

| Difficulty | Fast (quality 5) | Normal (quality 4) | Slow (quality 3) |
|------------|-------------------|--------------------|-------------------|
| Easy       | < 10 min          | 10–20 min          | > 20 min          |
| Medium     | < 15 min          | 15–30 min          | > 30 min          |
| Hard       | < 20 min          | 20–40 min          | > 40 min          |

These thresholds are hardcoded defaults, overridable in config via Viper (e.g. `thresholds.easy.fast: 10`).

**On failure:** quality is always 1, regardless of duration.

**On success (any quality):**
- Increment `repetition_count`.
- If `repetition_count == 1`: set `interval_days = 1`.
- If `repetition_count == 2`: set `interval_days = 6`.
- If `repetition_count >= 3`: set `interval_days = interval_days * easiness_factor`.
- Update `easiness_factor` using the SM-2 formula with the derived quality value.
- If `mastered_before == true` and this is the first attempt since the problem resurfaced from decay, aggressively boost `interval_days` (e.g. double it) so it doesn't reappear for months.
- If `interval_days > 60`, mark `mastered_before = true`.

**On failure:**
- Reset `repetition_count = 0`.
- Reset `interval_days = 1`.
- Update `easiness_factor` using the SM-2 formula with quality=1. Clamp EF to a minimum of 1.3.

**After both:** set `next_review_date = today + interval_days`.

### Initialization

New problems start with `easiness_factor = 2.5`, `interval_days = 1`, `repetition_count = 0`, `next_review_date = today`, `mastered_before = false`. They are immediately eligible for review.

### Implementation Note

The SM-2 easiness factor update is a single generic function: `EF' = EF + (0.1 - (5 - quality) * (0.08 + (5 - quality) * 0.02))`. It takes a `quality` parameter (0–5). Implement it as such — the quality value is derived from duration thresholds on success and hardcoded to 1 on failure.

### Practice session architecture

Practice sessions pre-compile the full queue at startup using `store.ListPickQueue`, which returns all due problems followed by upcoming problems in pick order. The queue is stored in the `PracticeModel` and popped as problems are completed — no further DB picks during a session.

Attempt timestamps (`started_at`) are stored in UTC and converted at query time with SQLite's `date(col, 'localtime')`. Scheduling dates (`next_review_date`) are stored in local time because they are date-only fields with no time component to convert — they must match `localToday()` comparisons directly.

When all due problems have been completed, the session enters free practice. Successful solves on upcoming problems record the attempt for stats but do not update SRS state (interval, EF, and next_review_date stay unchanged). Failures still reset SRS normally — a failure is evidence of forgetting regardless of schedule.

## Key Commands

- `hone` (no args) — open the stats dashboard / home screen
- `hone practice` — show start screen with due count, then launch the next problem
- `hone add` — parse a pasted URL to extract platform and slug, scrape the page, and create a problem entry
- `hone add -f FILE` — batch import from a flat URL list (one per line)
- `hone import FILE` — playlist-aware bulk import; `#Name` lines define playlist boundaries
- `hone export` — export problems grouped by playlist in human-readable format (round-trips with `hone import`)
- `hone export --backup` — full JSON dump of all data (problems, SRS state, attempts, playlists, config)
- `hone init BACKUPFILE` — restore from a `--backup` JSON file; only works if DB doesn't exist yet
- `hone playlist create|select|list` — manage playlists
- `hone auth [platform]` — save a browser session for scraping authenticated pages
- `hone stats` — statistics dashboard (same as no-args)
- `hone topics` — list/filter by topic

## Rod Usage

- **Platform registry**: Each platform (LeetCode, NeetCode, GeeksForGeeks) is a self-contained file in `internal/platform/` implementing the `Platform` interface. Platforms self-register via `init()`. Scraping selectors, monitor selectors, URL parsing, and wait strategies are all encapsulated per platform. Adding a new platform means creating one file — see `docs/dev/platforms.md`.
- **Practice sessions**: launches a visible (headful) Chrome window. Monitors the DOM for submission result indicators via the platform's `DetectResult()` method. Sends results back to the TUI via Go channels.
- **Scraping**: runs headless. The `add` command parses a pasted URL to identify the platform and extract the slug, then navigates to the page to scrape title, topics, difficulty, and other metadata. The scraper is a thin orchestrator; platform-specific extraction is in each platform file.
- **External Chrome launch**: The scraper launches Chrome via `exec.Command` (not Rod's `launcher.New()`) because Rod's launcher uses `--use-mock-keychain` and other flags that prevent Chrome from accessing the macOS Keychain, which is required to decrypt cookies saved during `hone auth`.
- **Browser reuse**: Batch and import operations share a single `scraper.Browser` instance across all URLs. Launching/killing Chrome per URL causes port exhaustion, profile lock corruption, and panics. Panic-prone Rod calls (`MustConnect`, `MustPage`) are wrapped in `rod.Try()`.
- **Platform-specific waits**: Platforms that need extra time after page load implement `ExtraWait()`. NeetCode sleeps 3s (client-side auth rendering). GeeksForGeeks sleeps 2s. LeetCode needs no extra wait.
- **Topic normalization**: Dashes in topic names are replaced with spaces for all platforms to prevent duplicates (e.g. "breadth-first search" → "breadth first search").

## TUI Design

- All views use lipgloss for styling and bubbles components where applicable.
- The statistics view renders tables and per-topic breakdowns.
- Forms for data entry (adding problems, creating playlists) use the huh library.
- Navigation between views is handled within the Bubble Tea program, treating the stats dashboard as the hub.
