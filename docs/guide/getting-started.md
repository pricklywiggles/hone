# Getting Started

## Requirements

- macOS (arm64 or amd64)
- Google Chrome installed (used for browser automation during practice sessions)

---

## Installation

/// tab | Homebrew
```sh
brew install pricklywiggles/hone/hone
```
///

/// tab | From source
Requires Go 1.21+.

```sh
git clone https://github.com/pricklywiggles/hone
cd hone
go install .
```
///

---

## First run

```sh
hone
```

This opens the dashboard and creates the database at `~/.local/share/hone/data.db` on first launch. The config file lives at `~/.config/hone/config.yaml` and is created automatically with sensible defaults.

---

## Adding your first problem

Paste a LeetCode or NeetCode URL:

```sh
hone import --url https://neetcode.io/problems/two-sum/question
```

hone opens a headless browser, scrapes the problem title, difficulty, and topics, then adds it to your library. The problem is immediately due for review.

### Batch import

If you have a list of URLs, use the playlist-aware import (see [Import & Export](import-export.md)):

```sh
hone import --playlist problems.txt
```

Or run `hone import` with no flags for a guided wizard.

---

## Starting a practice session

```sh
hone practice
```

hone shows a **start screen** with your active playlist (if any), how many problems are due today, and the total queue size. Press Enter to begin.

The first due problem opens in a visible Chrome window and hone starts a timer. Solve the problem and submit it — hone detects the submission result automatically and records the attempt.

When you're done, the TUI shows:

- Your result (pass/fail)
- Solve time and quality score
- The next scheduled review date
- **Today's stats** — solved, failed, and remaining counts for your playlist and overall

When all due problems are complete, hone shows a congratulations screen. You can continue with upcoming problems in **free practice** mode (SRS is frozen for successful solves) or quit and come back tomorrow.

---

## The dashboard

Running `hone` with no arguments opens the tabbed dashboard:

```
Stats │ Problems │ Playlists │ Topics
```

- **Stats** — overview counts, streak, progress bars by difficulty, per-topic breakdown
- **Problems** — full list with SRS state, sortable and filterable
- **Playlists** — create and manage playlists; press Enter to set as active filter
- **Topics** — topic breakdown; press Enter to filter practice to that topic

Press `p` from any tab to start a practice session using the active filter.

---

## What's stored

| Path | Contents |
|------|----------|
| `~/.local/share/hone/data.db` | SQLite database (problems, SRS state, attempts, playlists) |
| `~/.config/hone/config.yaml` | Config (thresholds, platform URL templates) |
| `~/.local/share/hone/browser-profile/` | Persistent Chrome profile (keeps you logged in to problem sites) |

---

## Logging in to problem sites

The Chrome profile is reused across sessions, so you only need to log in once. Close Google Chrome before running auth:

```sh
hone auth neetcode   # opens a browser window — log in, then press Enter
hone auth leetcode
```

After authenticating, practice sessions will have access to your account (required for submission detection on some platforms).

!!! info "Please note:"
    hone does **not** store or transmit any credentials. All authentication is handled exclusively through the browser, exactly as you would log in normally.
