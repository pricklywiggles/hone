# Configuration

hone stores its config in a platform-specific location. The file is created automatically on first run with sensible defaults. You can edit it directly to customize thresholds and platform URL templates.

---

## File locations

| | macOS / Linux | Windows |
|------|---------------|---------|
| **Config** | `~/.config/hone/config.yaml` | `%AppData%\hone\config.yaml` |
| **Database** | `~/.local/share/hone/data.db` | `%AppData%\hone\data.db` |
| **Chrome profile** | `~/.local/share/hone/browser-profile/` | `%AppData%\hone\browser-profile\` |

---

## Quality thresholds

Quality is derived automatically from your solve time. The thresholds define what counts as "fast", "normal", and "slow" for each difficulty:

```yaml
thresholds:
  easy:
    fast: 10    # minutes — quality 5 if faster than this
    normal: 20  # minutes — quality 4 if between fast and normal; quality 3 if slower
  medium:
    fast: 18
    normal: 30
  hard:
    fast: 30
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
  geeksforgeeks:
    url_template: "https://www.geeksforgeeks.org/problems/{{slug}}/1"
```

The `{{slug}}` placeholder is replaced with the problem slug (e.g. `two-sum`).

---

## Active filter

The active filter (playlist or topic) is stored in the database, not in config.yaml. Set it from the dashboard (Playlists or Topics tab, press Enter) or via the CLI:

```sh
hone playlist select "Week 1"
```

Playlist and topic are mutually exclusive — setting one clears the other. The active filter is included in JSON backups and restored automatically.

---

## Full default config

```yaml
thresholds:
  easy:
    fast: 10
    normal: 20
  medium:
    fast: 18
    normal: 30
  hard:
    fast: 30
    normal: 40

platforms:
  leetcode:
    url_template: "https://leetcode.com/problems/{{slug}}/"
  neetcode:
    url_template: "https://neetcode.io/problems/{{slug}}/question"
  geeksforgeeks:
    url_template: "https://www.geeksforgeeks.org/problems/{{slug}}/1"
```
