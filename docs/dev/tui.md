# TUI Development

hone's TUI is built with [Bubble Tea](https://github.com/charmbracelet/bubbletea). This page covers the patterns used throughout `internal/tui/`.

---

## Router stack

Navigation uses a stack of `tea.Model` values managed by `router.go`.

```go
// Push a new view onto the stack
return m, tui.Push(tui.NewAddModel(db, profileDir, ""))

// Pop back to the previous view
return m, tui.Pop()
```

The router receives all messages. It forwards them to the top-of-stack model and handles `PushMsg` / `PopMsg` to transition between views.

When a model exits (e.g. user presses `q`), it emits `Pop()`. When it wants to navigate forward, it emits `Push(newModel)`. No model holds a reference to its parent.

---

## Standalone vs embedded models

Some models can run both as a full standalone program and embedded inside a parent model. The `AddModel` is an example:

```go
type AddModel struct {
    standalone bool
    // ...
}

func NewAddModel(db *sqlx.DB, profileDir, prefill string) AddModel {
    return AddModel{standalone: true, ...}
}

func (m AddModel) exit() tea.Cmd {
    if m.standalone {
        return tea.Quit
    }
    return Pop()
}
```

When embedded (e.g. opened from the playlist picker), `standalone` is set to `false` and exit pops back to the parent instead of quitting.

---

## Communicating between models

Models communicate via typed message values. When `AddModel` successfully adds a problem, it emits `problemAddedMsg{}`. The parent model handles this in its `Update`:

```go
case problemAddedMsg:
    return m, m.loadCmd() // reload the problem list
```

This is the standard Bubble Tea pattern — no callbacks, no shared mutable state.

---

## Running a program

Two helpers in `run.go`:

```go
// Full-screen (altscreen) — used for the main dashboard
tui.Run(model)

// Inline — output scrolls in the terminal, visible after exit
// Used for hone import (progress output)
tui.RunInline(model)
```

---

## `colorTable`

Use `colorTable` from `color_table.go` whenever table cells contain lipgloss-styled text:

```go
t := newColorTable([]colorColumn{
    {header: "Title", width: 40},
    {header: "Difficulty", width: 10},
})
for _, row := range problems {
    t.addRow([]string{
        row.Title,
        difficultyStyle.Render(row.Difficulty),
    })
}
view := t.render()
```

`colorTable` uses `lipgloss.Width()` for all cell measurements, which correctly handles ANSI escape sequences. Never use `bubbles/table` for styled content.

---

## Key maps

All key bindings are defined in `keys.go` as `key.Map` structs. Each model has its own map. They're passed to a `bubbles/help` model for the help bar:

```go
type statsKeys struct {
    Practice key.Binding
    Add      key.Binding
    Help     key.Binding
    Quit     key.Binding
}

func (k statsKeys) ShortHelp() []key.Binding {
    return []key.Binding{k.Practice, k.Add, k.Help, k.Quit}
}
```

---

## List sizing

When the terminal resizes, all `bubbles/list` models need their height updated. The pattern is a `resizeList` method that computes the available height:

```go
func (m *MyModel) resizeList(height int) {
    const reserved = 3 // blank line + content line + help line
    m.list.SetHeight(height - reserved)
}
```

Called in `Update` when a `tea.WindowSizeMsg` arrives.

---

## Wizard models

`ImportWizardModel` and `ExportWizardModel` are multi-step flows built from `bubbles/list`, `bubbles/filepicker`, `bubbles/textinput`, and `bubbles/spinner`. Each wizard uses an integer state enum (e.g. `iwChooseType`, `iwChooseFile`, `iwRunning`, `iwDone`) and switches on it in `Update`/`View`.

Key patterns:

- **Shared helpers**: `wizardItem` (implements `list.Item`) and `newWizardList` create consistent selection lists across both wizards, styled with `colorAccent`.
- **Back navigation**: Pressing Escape moves to the previous step. The export wizard's `handleBack()` centralizes this logic.
- **Delegation**: Once the user finishes choosing options, the wizard delegates to existing models. The import wizard embeds `ImportModel` for playlist imports and `AddModel` for single-URL adds. The export wizard runs backup/playlist export functions directly.
- **Inline mode**: Both wizards run via `tui.RunInline` so output remains visible in the terminal after exit, matching the existing import progress pattern.

---

## Splash screen

`SplashModel` wraps the router and shows the ASCII art intro before the first `WindowSizeMsg` is forwarded to the inner model. The inner model's `Init()` is called once the splash finishes, so it's correctly sized when activated.

The gradient animation uses per-character HSL coloring:

```go
hue := math.Mod(float64(col)*1.5+float64(frame)*3, 360)
color := hslHex(hue, 1.0, 0.65)
```

`frame` is incremented on each 50ms tick, sweeping the hue across the full rainbow.
