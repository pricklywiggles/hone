# Learning

## Spaced repetition

Spaced repetition is a study technique that schedules reviews at increasing intervals. The core insight: reviewing something just before you forget it is more efficient than reviewing it on a fixed schedule. Problems you know well are shown infrequently; problems you struggle with appear often until they stick.

hone uses a modified **SM-2** algorithm, the same algorithm behind Anki and many other SRS tools.

---

## The SM-2 algorithm

Each problem has an **easiness factor (EF)**, an **interval** (days until next review), and a **repetition count** (consecutive successes).

After every attempt, hone:

1. Derives a **quality score** (1–5) from the result and solve time
2. Updates the EF using the SM-2 formula
3. Computes a new interval based on the repetition count and EF
4. Schedules `next_review_date = today + interval`

### Quality scoring

Quality is never self-reported. hone derives it automatically:

| Result | Quality |
|--------|---------|
| Failed | 1 |
| Solved slowly (≥ normal threshold) | 3 |
| Solved at normal pace | 4 |
| Solved quickly (< fast threshold) | 5 |

Thresholds are per-difficulty and configurable (see [Configuration](configuration.md)).

### Interval progression

On each consecutive success:

| Repetition | Interval |
|-----------|----------|
| 1st success | 1 day |
| 2nd success | 6 days |
| 3rd+ success | previous interval × EF |

On failure, the interval resets to 1 day and the repetition count resets to 0.

### Easiness factor

The EF starts at 2.5 and is updated after every attempt:

```
EF' = EF + (0.1 - (5 - q) × (0.08 + (5 - q) × 0.02))
```

Where `q` is the quality score (1–5). The EF is clamped to a minimum of 1.3 — problems never schedule faster than approximately weekly at steady state.

A higher EF means longer intervals (the problem is "easy" for you). A lower EF means shorter intervals (needs more practice).

---

## Mastered problems

When a problem's interval exceeds 60 days, it's marked `mastered_before = true`. This is a one-way flag — once mastered, always considered mastered for stats purposes.

If a mastered problem is failed (EF decays, interval resets), it resurfaces for re-practice. Once you succeed on it twice in a row again, hone recognizes you still know it and applies an **interval boost** (doubling the computed interval), so it doesn't clog your queue for months.

---

## The picker

`hone practice` selects the next problem using:

1. **Filter** — if a playlist or topic is active, only consider those problems
2. **Due problems** — all problems where `next_review_date ≤ today`
3. **Pick rule** — if due problems exist, pick the most overdue (oldest date first); ties break by difficulty (easy first), then playlist position or random. Otherwise pick the one with the nearest upcoming date

The TUI makes the distinction clear: "you have 5 problems due" vs "nothing due today — here's one in 3 days."

### Free practice

When all due problems have been completed, hone shows a congratulations screen and enters **free practice** mode. You can keep practicing upcoming problems, but SRS is frozen for successful solves — the attempt is recorded for stats, but the interval, EF, and next review date stay unchanged.

The reasoning: spaced repetition works because of the spacing. Solving a problem three times in one sitting proves you can hold it in working memory, not that you've built durable recall. Inflating intervals from same-day re-solves would undermine the scheduling.

**Failures still reset SRS.** If you fail an upcoming problem, hone resets its SRS state normally (interval back to 1, repetition count to 0) because a failure is genuine evidence of forgetting regardless of when the problem was scheduled.

---

## New problems

When a problem is first added, it gets:

- `easiness_factor = 2.5`
- `interval_days = 1`
- `repetition_count = 0`
- `next_review_date = today`

It's immediately due for review.
