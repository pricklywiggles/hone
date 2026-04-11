# hone

**Practice coding problems with spaced repetition from your terminal.**

hone is a macOS CLI that tracks your LeetCode, NeetCode, and GeeksForGeeks sessions, schedules problems using the SM-2 algorithm, and keeps you practicing the right things at the right time. Open a problem in your browser, submit it, and hone records the result automatically.

<br><br>

<video autoplay loop muted playsinline>
  <source src="assets/splash.webm" type="video/webm">
  <img src="assets/splash.gif" alt="hone demo">
</video>

---

## How it works

```
hone                             # open the dashboard
hone practice                    # pick and launch the next due problem
hone add <url>                   # add a single problem by URL
hone import --playlist file.txt  # bulk import with playlist grouping
hone import                      # guided import wizard
```

The **spaced repetition engine** uses a modified SM-2 algorithm to schedule reviews at increasing intervals. After each attempt, hone derives a quality score from your solve time — no self-reporting needed. Solve quickly and the interval grows; solve slowly and it stays shorter; fail and it resets to tomorrow. Problems you've mastered fade into the background, while problems you struggle with keep coming back until they stick.

When you finish all due problems in a session, you can keep practicing — hone enters **free practice** mode and serves upcoming problems without advancing their SRS state, so your intervals stay honest. [Learn more →](guide/concepts.md)

---

## Quick start

/// tab | Homebrew
```sh
brew install pricklywiggles/hone/hone
```
///

/// tab | From source
```sh
git clone https://github.com/pricklywiggles/hone
cd hone
go install .
```
///

Then add your first problem and start practicing:

```sh
hone add https://neetcode.io/problems/two-sum/question
hone practice
```

---

## Features

- **Automatic result detection** — hone monitors your browser for submission results; no manual input needed
- **SM-2 scheduling** — quality is derived from your solve time, not self-reported ratings
- **Playlist grouping** — organize problems by topic, week, or any other dimension
- **Stats dashboard** — tabbed TUI with per-topic progress, streaks, and due counts
- **Bulk import/export** — import from a URL list file; export + restore with full JSON backup
- **Zero infrastructure** — single SQLite file, no accounts, no network access beyond the problem sites

---

## Navigation

| Section | What's there |
|---------|-------------|
| [Getting Started](guide/getting-started.md) | Installation, first run, adding problems |
| [Commands Reference](guide/commands.md) | Every command with examples |
| [Dashboard Guide](guide/dashboard.md) | Tabs, shortcuts, filters |
| [Import & Export](guide/import-export.md) | File format, backup/restore |
| [Configuration](guide/configuration.md) | Config file, thresholds, platform templates |
| [Learning](guide/concepts.md) | How spaced repetition and the SM-2 algorithm work in hone |
| [Architecture](dev/architecture.md) | Package map and data flow |
