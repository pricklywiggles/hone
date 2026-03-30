# Configuration

hone stores its config at `~/.config/hone/config.yaml`. The file is created automatically on first run. You can edit it directly or let hone manage it through the UI (active filter selections are written by the app).

---

## File location

| Path | Purpose |
|------|---------|
| `~/.config/hone/config.yaml` | Config file |
| `~/.local/share/hone/data.db` | SQLite database |
| `~/.local/share/hone/browser-profile/` | Chrome profile for browser automation |

---

## Quality thresholds

Quality is derived automatically from your solve time. The thresholds define what counts as "fast", "normal", and "slow" for each difficulty:

```yaml
thresholds:
  easy:
    fast: 10    # minutes — quality 5 if faster than this
    normal: 20  # minutes — quality 4 if between fast and normal; quality 3 if slower
  medium:
    fast: 15
    normal: 30
  hard:
    fast: 20
    normal: 40
```

| Solve time | Quality |
|-----------|---------|
| < fast threshold | 5 (great) |
| fast ≤ time < normal | 4 (good) |
| ≥ normal threshold | 3 (slow) |
| Failed | 1 (always) |

---

## Platform URL templates

These templates define how problem URLs are constructed from a platform + slug. Modify them if a platform changes its URL structure.

```yaml
platforms:
  leetcode:
    url_template: "https://leetcode.com/problems/{{slug}}/"
  neetcode:
    url_template: "https://neetcode.io/problems/{{slug}}/question"
```

The `{{slug}}` placeholder is replaced with the problem slug (e.g. `two-sum`).

---

## Active filter

The active filter is written by hone when you select a playlist or topic from the dashboard. You can also set it manually:

```yaml
active_playlist_id: 3   # ID of the active playlist (0 = none)
active_topic_id: 0      # ID of the active topic (0 = none)
```

Playlist and topic are mutually exclusive — setting one clears the other.

---

## Full default config

```yaml
thresholds:
  easy:
    fast: 10
    normal: 20
  medium:
    fast: 15
    normal: 30
  hard:
    fast: 20
    normal: 40

platforms:
  leetcode:
    url_template: "https://leetcode.com/problems/{{slug}}/"
  neetcode:
    url_template: "https://neetcode.io/problems/{{slug}}/question"

active_playlist_id: 0
active_topic_id: 0
```
