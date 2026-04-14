# Dashboard Guide

Running `hone` with no arguments opens the tabbed dashboard. This is the main interface for tracking your progress and navigating to all other features.

---

## Tab overview

```
 Stats │ Problems │ Playlists │ Topics
```

Switch tabs with `Tab` / `Shift+Tab` or the left/right arrow keys.

---

## Stats tab

The Stats tab is the landing screen. It shows:

- **Overview cards** — total problems, due today, mastered count, current streak
- **Progress bars** — segmented bars showing mastered/attempted split per difficulty (Easy / Medium / Hard), with color legend
- **Worst performance by playlist** — playlists ranked by (successes − failures) / total problems; only shown for playlists with attempts
- **Worst performance by topic** — topics ranked the same way; bars show succeeded/failed/unattempted segments
- **Currently practicing** — shown when a playlist or topic filter is active; displays filter name with due count

---

## Problems tab

A sortable, filterable table of all problems in your library.

| Column | Description |
|--------|-------------|
| Title | Problem name |
| Difficulty | easy / medium / hard |
| Platform | leetcode / neetcode / geeksforgeeks |
| Next review | Scheduled date (highlighted if overdue) |
| Attempts | Total attempt count |
| Successes | Number of successful submissions |

**Shortcuts:**

| Key | Action |
|-----|--------|
| `a` | Add a new problem |
| `/` | Filter by title |
| `s` | Cycle sort order |
| `Enter` | Open problem in browser |

---

## Playlists tab

Lists all playlists with problem counts.

**Shortcuts:**

| Key | Action |
|-----|--------|
| `Enter` | Toggle playlist as active filter (select/deselect) |
| `a` | Open problem picker to add problems to a playlist |
| `n` | Create a new playlist |

The active playlist is shown with a `*` marker and sets the filter for practice sessions and the Stats tab. Selecting a playlist clears any active topic filter.

### Problem picker

Pressing `a` on a playlist opens a multi-select picker showing all problems. Toggle items with `Space`, confirm with `Enter`. From the picker, press `a` to open the add-problem flow if the problem you want isn't in your library yet.

---

## Topics tab

Lists all topics derived from your problem library.

**Shortcuts:**

| Key | Action |
|-----|--------|
| `Enter` | Toggle topic as active filter |
| `s` | Cycle sort order (alphabetical / % mastered / weakest first) |

The active topic is shown with a `*` marker. Selecting a topic clears any active playlist filter.

---

## Active filter indicator

When a filter is active, the tab bar shows it:

```
 Stats │ Problems │ Playlists │ Topics    [Favorites ▸ 3 due]
```

The filter applies to:

- Practice sessions started with `p`
- The "currently practicing" section on the Stats tab
- The due count shown in the indicator

---

## Splash screen

The first time hone opens (or after a cold start), it shows an animated ASCII art splash screen. Press any key or wait for it to advance automatically.

---

## Help bar

Press `?` to toggle the help bar at the bottom of the screen. It shows all available shortcuts for the current tab.
