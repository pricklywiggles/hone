<p align="center">
  <img src="docs/assets/splash.gif" alt="hone demo" width="600">
</p>

**Practice coding problems with spaced repetition from your terminal.**

hone schedules LeetCode, NeetCode, and GeeksForGeeks problems using the SM-2 algorithm, opens them in your browser, detects submission results automatically, and tells you exactly when to revisit each one. No self-reporting, no manual ratings — just solve the problem and hone tracks the rest.

If you find this useful, buy me a coffee!

[![ko-fi](https://ko-fi.com/img/githubbutton_sm.svg)](https://ko-fi.com/L3L81X4KW7)

---

## Documentation

Full docs at **https://pricklywiggles.github.io/hone** — user guide, command reference, SM-2 concepts, and developer documentation in one place.

---

## Install

```sh
brew install pricklywiggles/hone/hone
```

Or build from source (requires Go 1.21+):

```sh
git clone https://github.com/pricklywiggles/hone
cd hone && go install .
```

---

## Quick start

```sh
# Add a problem
hone add https://neetcode.io/problems/two-sum/question

# Start practicing — shows queue summary, then opens problems in Chrome
hone practice

# Open the stats dashboard
hone
```

---

## How it works

Each problem has an **easiness factor** and a **review interval** that update after every attempt:

- Solved quickly → longer interval, higher EF
- Solved slowly → moderate interval
- Failed → resets to 1 day, EF decays

Quality is derived from your solve time against per-difficulty thresholds — no self-rating. hone monitors your browser for submission results so you never have to switch back to the terminal.

When a problem's interval exceeds 60 days it's marked mastered. If it resurfaces and you nail it twice in a row, hone recognizes you still know it and schedules it far out again.

---

## Dashboard

```sh
hone
```

A tabbed TUI with four views:

| Tab | What's there |
|-----|-------------|
| **Stats** | Overview counts, streak, progress by difficulty, per-topic breakdown |
| **Problems** | Full library with SRS state, sortable and filterable |
| **Playlists** | Create and manage groups; Enter toggles as active filter |
| **Topics** | Topic breakdown with sort modes; Enter filters practice to that topic |

Press `p` from any tab to start a practice session on the current filter. Use `←` / `→` or `Tab` to switch tabs.

---

## Commands

| Command | Description |
|---------|-------------|
| `hone` | Stats dashboard |
| `hone practice` | Start a practice session with start screen and queue summary |
| `hone add <url>` | Add a problem by URL |
| `hone import` | Guided import wizard |
| `hone import --playlist file.txt` | Playlist-aware bulk import |
| `hone import --backup backup.json` | Restore from a JSON backup |
| `hone export` | Guided export wizard |
| `hone export --playlist` | Export all playlists in text format |
| `hone export --playlist NAME` | Export a single playlist by name |
| `hone export --backup` | Full JSON backup (SRS state, attempts, playlists) |
| `hone playlist create\|list\|select` | Manage playlists |
| `hone auth neetcode\|leetcode\|geeksforgeeks` | Save a browser session for a platform |

---

## Import format

```
# Favorites
https://neetcode.io/problems/two-sum/question
https://neetcode.io/problems/valid-anagram/question

# Week 1
https://neetcode.io/problems/climbing-stairs/question
```

`# Name` headers define playlist boundaries. Problems before any header are imported without a playlist. Existing problems are skipped (not re-scraped) but still added to the playlist. Use `hone import --playlist file.txt` to import, or just `hone import` for the guided wizard.

---

## Backup and restore

```sh
# On old machine
hone export --backup -o hone-backup.json

# On new machine
hone import --backup hone-backup.json
```

The JSON backup includes all problems, SRS state, attempt history, and playlists.

---

## Data

| Path | Contents |
|------|----------|
| `~/.local/share/hone/data.db` | SQLite database |
| `~/.config/hone/config.yaml` | Config (thresholds, active filter, URL templates) |
| `~/.local/share/hone/browser-profile/` | Persistent Chrome profile |

Reset the database: `rm ~/.local/share/hone/data.db` (recreated on next run).

---

## Configuration

Quality thresholds and platform URL templates are overridable in `~/.config/hone/config.yaml`:

```yaml
thresholds:
  easy:   { fast: 10, normal: 20 }   # minutes
  medium: { fast: 15, normal: 30 }
  hard:   { fast: 20, normal: 40 }
```

---

## License

MIT
