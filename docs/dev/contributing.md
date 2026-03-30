# Contributing

## Prerequisites

- Go 1.21+
- macOS (browser automation uses macOS-specific Chrome paths)
- Chrome or Chromium

---

## Build

```sh
git clone https://github.com/pricklywiggles/hone
cd hone
go build ./...     # compile everything
go run .           # run the CLI (creates ~/.local/share/hone/data.db on first run)
go test ./...      # run all tests
go vet ./...       # static analysis
```

---

## Project structure

```
cmd/          Cobra command definitions — keep thin, delegate to internal/
internal/     All application logic — no main package imports allowed here
docs/         MkDocs documentation source
```

Commands in `cmd/` should be ~50 lines: parse args, call `internal/` packages, wire results to TUI or stdout. Business logic belongs in `internal/`.

---

## Code style

- Standard `gofmt` formatting — run before committing
- No comments that restate what the code does — only explain *why*
- No commented-out code — delete it
- No doc headers on unexported functions
- Table-driven tests for any pure function with multiple input cases

---

## Working with the database

Reset the database during development:

```sh
rm ~/.local/share/hone/data.db
go run .   # recreated with fresh migrations
```

For schema changes, add a new goose migration file in `internal/db/migrations/`. Do not edit existing migrations. The filename convention is `NNN_description.sql`.

---

## Adding a feature

1. **Start in `internal/`** — write the logic with tests before wiring it to a command
2. **Add the command in `cmd/`** — keep it thin; call the `internal/` package
3. **Add a TUI model in `internal/tui/`** if the command needs interactive output
4. **Update `docs/`** — add or update the relevant documentation page
5. **Update `ROADMAP.md`** — mark the phase complete with a short summary

---

## Documentation

```sh
pip install mkdocs-material mkdocs-minify-plugin
mkdocs serve        # live preview at http://127.0.0.1:8000
mkdocs build --strict   # fails on broken links (same check as CI)
```

---

## PR checklist

- [ ] `go test ./...` passes
- [ ] `go vet ./...` clean
- [ ] New packages/functions have table-driven tests where applicable
- [ ] Documentation updated if user-facing behavior changed
- [ ] No debug prints or commented-out code
