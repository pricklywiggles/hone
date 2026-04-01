# Commands Reference

## `hone`

Opens the statistics dashboard (the default landing screen).

```sh
hone
```

---

## `hone practice`

Picks the next due problem and opens it in a browser window.

```sh
hone practice
```

The picker respects the active filter (playlist or topic). If nothing is due today, hone picks the problem with the nearest upcoming review date and tells you how many days early you are.

**Flow:**

1. Pick → display problem info (title, difficulty, topics, due status)
2. Open problem URL in Chrome
3. Start timer
4. Detect submission result from the DOM
5. Compute quality from result + solve time
6. Update SRS state and record attempt
7. Show summary (result, time, next review)

---

## `hone add`

Add a problem by URL. hone scrapes metadata (title, difficulty, topics) and inserts the problem with default SRS state (due today).

```sh
hone add https://neetcode.io/problems/two-sum/question
hone add https://leetcode.com/problems/climbing-stairs/
```

Supported platforms: **NeetCode**, **LeetCode**, **GeeksForGeeks**.

### Batch add

```sh
hone add -f problems.txt
```

The file should contain one URL per line. Lines starting with `#` or `//` are ignored. For playlist-aware import, use `hone import` instead.

---

## `hone import`

Import problems from a file with optional playlist grouping.

```sh
hone import my-list.txt
```

The file format uses `# Name` headers to define playlist boundaries:

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

---

## `hone export`

Export problems to stdout or a file.

```sh
# Playlist format (compatible with hone import)
hone export
hone export -o my-list.txt

# Full JSON backup
hone export --backup
hone export --backup -o backup.json
```

The default export groups problems by playlist under `# Name` headers. Problems not in any playlist appear at the top. The output round-trips with `hone import`.

The `--backup` format includes SRS state, attempt history, and playlist memberships — everything needed to fully restore your data on another machine.

---

## `hone init`

Restore from a JSON backup created by `hone export --backup`.

```sh
hone init backup.json
```

!!! warning
    This command refuses to run if a database already exists at `~/.local/share/hone/data.db`. To start fresh from a backup, run: `rm ~/.local/share/hone/data.db && hone init backup.json`

On success, prints a summary:

```
restored 47 problem(s), 3 playlist(s), 218 attempt(s)
```

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
