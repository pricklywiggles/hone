# Commands Reference

## `hone`

Opens the statistics dashboard (the default landing screen).

```sh
hone
```

---

## `hone practice`

Shows a start screen with your active playlist, due count, and queue size, then picks the next problem and opens it in a browser window.

```sh
hone practice
```

The picker respects the active filter (playlist or topic). If nothing is due today, the start screen tells you and offers free practice on upcoming problems.

**Flow:**

1. Show start screen (playlist/topic name, due count, queue size)
2. Press Enter → open the first problem in Chrome, start timer
3. Detect submission result from the DOM
4. Compute quality from result + solve time
5. Update SRS state and record attempt
6. Show done card (result, time, quality, next review, today's stats)
7. Press Enter → next problem (or quit)

When all due problems are completed, a congratulations screen appears. You can continue into free practice (SRS frozen for successful solves) or quit.

---

## `hone import`

Import problems or restore from a backup. Without flags, launches a guided wizard that walks you through the options.

```sh
# Guided wizard
hone import

# Add a single problem by URL
hone import --url https://neetcode.io/problems/two-sum/question

# Playlist-aware import from text file
hone import --playlist my-list.txt

# Restore from JSON backup
hone import --backup backup.json
```

Supported platforms: **NeetCode**, **LeetCode**, **GeeksForGeeks**.

### Playlist file format

The `--playlist` flag expects a text file with `# Name` headers defining playlist boundaries:

```
# Favorites
https://neetcode.io/problems/two-sum/question
https://neetcode.io/problems/valid-anagram/question

# Week 1
https://neetcode.io/problems/climbing-stairs/question
```

**Behavior:**

- If a playlist doesn't exist, it's created.
- If a problem already exists in your library, it's skipped (not re-scraped) but still added to the playlist.
- Problems before any `#` header are imported without a playlist.
- Lines starting with `//` or blank lines are ignored.

Progress is shown inline as each URL is processed.

### Backup restore

The `--backup` flag restores from a JSON backup created by `hone export --backup`.

!!! warning
    This refuses to run if a database already exists at `~/.local/share/hone/data.db`. To start fresh from a backup, run: `rm ~/.local/share/hone/data.db && hone import --backup backup.json`

---

## `hone export`

Export problems or create backups. Without flags, launches a guided wizard.

```sh
# Guided wizard
hone export

# All playlists in text format
hone export --playlist
hone export --playlist -o my-list.txt

# Single playlist by name
hone export --playlist "Week 1"

# Full JSON backup
hone export --backup
hone export --backup -o backup.json
```

The `--playlist` format groups problems under `# Name` headers. The output round-trips with `hone import --playlist`.

The `--backup` format includes SRS state, attempt history, and playlist memberships — everything needed to fully restore your data on another machine.

---

## `hone playlist`

Manage playlists.

```sh
hone playlist create "Week 1"
hone playlist list
hone playlist select "Week 1"
```

You can also manage playlists from the Playlists tab in the dashboard.

---

## `hone stats`

Alias for `hone` — opens the statistics dashboard.

---

## `hone topics`

Opens the dashboard at the Topics tab.

---

## `hone auth`

Open a browser window to log in to a problem platform. The session is saved in the browser profile and reused for all future sessions. Google Chrome must be closed before running this command.

```sh
hone auth neetcode
hone auth leetcode
hone auth geeksforgeeks
```

---

## Global flags

| Flag | Description |
|------|-------------|
| `--help` | Show help for any command |

---

## Keyboard shortcuts (dashboard)

| Key | Action |
|-----|--------|
| `Tab` / `Shift+Tab` | Next / previous tab |
| `←` / `→` | Same as Tab / Shift+Tab |
| `p` | Start practice session |
| `a` | Add a problem (Stats and Problems tabs) |
| `q` / `Ctrl+C` | Quit |
| `?` | Toggle help bar |

Additional per-tab shortcuts are shown in the help bar at the bottom of each tab.
