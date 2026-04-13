# Import & Export

hone supports two data formats: a **playlist format** for sharing problem lists, and a **JSON backup format** for full data portability. Both `hone import` and `hone export` offer a guided wizard when run without flags.

---

## Playlist format

The playlist format is a plain text file. It's designed to be human-readable and easy to edit.

```
# Favorites
https://neetcode.io/problems/two-sum/question
https://neetcode.io/problems/valid-anagram/question

# Week 1
https://neetcode.io/problems/climbing-stairs/question
https://leetcode.com/problems/house-robber/

https://neetcode.io/problems/coin-change/question
```

**Rules:**

- `# Name` — starts a playlist section; all URLs below belong to that playlist
- URLs before any `#` header are imported without a playlist
- Blank lines and `//` comments are ignored
- Any URL order is preserved within each section
- A `#` header with no URLs below it is silently skipped

### Importing

```sh
hone import --playlist my-list.txt
```

hone processes URLs sequentially. For each URL:

- If the problem doesn't exist in your library — scrape and insert it
- If it does exist — skip scraping, but still add it to the playlist
- If the playlist doesn't exist — create it

Progress is shown inline as each URL completes:

```
  Importing 5 problems across 2 playlist(s)

  ✓  neetcode/two-sum  [Favorites]
  ✓  neetcode/valid-anagram  [Favorites]
  –  neetcode/climbing-stairs  already in library
  ✓  leetcode/house-robber  [Week 1]
  ✗  neetcode/coin-change  scrape: timeout

  ████████████░░░░░░░░░░░░░░░░░░░░░░░░░░░░  4 / 5

  ✓ 3 added  – 1 skipped  ✗ 1 failed  1 playlist(s) created
```

### Exporting

```sh
# All playlists
hone export --playlist
hone export --playlist -o my-list.txt

# Single playlist by name
hone export --playlist "Week 1"
```

The output groups problems by playlist and round-trips cleanly with `hone import --playlist`.

---

## JSON backup format

The `--backup` flag produces a complete snapshot of your database — problems, SRS state, attempt history, and playlists.

```sh
hone export --backup -o backup.json
```

Example output:

```json
{
  "version": 2,
  "exported_at": "2026-03-30T14:00:00Z",
  "active_playlist": "Favorites",
  "problems": [
    {
      "platform": "neetcode",
      "slug": "two-sum",
      "title": "Two Sum",
      "difficulty": "easy",
      "created_at": "2026-03-01 09:00:00",
      "topics": ["array", "hash table"],
      "easiness_factor": 2.6,
      "interval_days": 6,
      "repetition_count": 2,
      "next_review_date": "2026-04-05",
      "mastered_before": false
    }
  ],
  "playlists": [
    {
      "name": "Favorites",
      "problems": ["neetcode/two-sum", "neetcode/valid-anagram"]
    }
  ],
  "attempts": [
    {
      "problem": "neetcode/two-sum",
      "started_at": "2026-03-28 10:00:00",
      "completed_at": "2026-03-28 10:08:30",
      "result": "success",
      "duration_seconds": 510,
      "quality": 5
    }
  ]
}
```

The `active_playlist` and `active_topic` fields record which filter was active at export time (by name). They are omitted when no filter is active. On restore, hone looks up the name and re-activates it.

### Restoring

```sh
hone import --backup backup.json
```

!!! warning "Database must not exist"
    `hone import --backup` refuses to run if `~/.local/share/hone/data.db` already exists, to prevent accidental data loss. Delete the database first, then restore:

`rm ~/.local/share/hone/data.db && hone import --backup backup.json`

---

## Moving to a new machine

1. On the old machine:
   ```sh
   hone export --backup -o hone-backup.json
   ```

2. Transfer `hone-backup.json` to the new machine (AirDrop, scp, etc.)

3. On the new machine:
   ```sh
   brew install pricklywiggles/hone/hone
   hone import --backup hone-backup.json
   ```

Your entire history, SRS state, playlists, and active filter selection are restored. You'll need to re-authenticate with problem platforms (`hone auth neetcode`), since the browser profile is not included in the backup.
