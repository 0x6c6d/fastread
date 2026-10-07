# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project

`fastread`: Linux CLI in Go for RSVP speed reading (one word at a time, red fixed focus letter, TUI and Gio GUI). No code exists yet; the spec is `README.md`, and the loop turns it into `Plan.md` and `Tasks.md`. Once code exists, keep the commands and layout below in sync with it.

## Commands

```
go build ./...                      # full build (GUI needs cgo + Gio system libs)
go build -tags nogui ./...          # TUI-only build, must also work with CGO_ENABLED=0
gofmt -l .                          # must print nothing
go vet ./...
go test ./...                       # also run with -race and with -tags nogui
```

Combined check (exit 0 = healthy): `test -z "$(gofmt -l .)" && go vet ./... && go test ./... && go test -tags nogui ./... && go build ./... && go build -tags nogui ./...`

## Architecture

- `cmd/fastread`: flag parsing, wiring, exit codes (0 ok, 1 runtime error, 2 usage error).
- `internal/{input,tokenize,orp,timing,state}`: UI-independent logic, no UI imports. `timing.Delay` is a pure function.
- `internal/tui`: terminal rendering and key handling, golden-tested frames.
- `internal/gui`: the only package importing Gio (`gioui.org`); layout math is a pure function tested without a display. Excluded by the `nogui` build tag.
- Resume state lives in `$XDG_STATE_HOME/fastread/` (fallback `~/.local/state/fastread/`).
- Usage docs go under `docs/`; `README.md` is the protected spec.

## Delegation workflow: supervise, don't implement

For non-trivial implementation work, use the supervisor/worker loop (`/loop-coding`, full procedure in `.claude/skills/loop-coding/SKILL.md`) instead of implementing directly:

- Project flow: `README.md` (spec) → `prompts/generate-plan.md` → `Plan.md` → `prompts/generate-tasks.md` → `Tasks.md` (small index) + `tasks/<ID>.md` (brief) + `scripts/verify/<ID>.sh` (verification) → loop runs the index row by row and reads a task file only when it dispatches that task.
- Done = a verification script that exits 0, never "looks right"; `scripts/check-protected.sh` proves the spec, prompts, skill and this section are untouched; keep the loop alive with `/goal`.
- One fresh worker per task, `model` per task (haiku / sonnet / opus); the supervisor verifies and commits itself and never trusts the worker's report.
- Fail → concrete feedback to the same worker; stuck after 3 rounds → new worker one tier up with a handoff; max 8 rounds, then `blocked` and ask the user.
- Friction in the loop itself (skill, prompts, spec template) → log it in `Improvements.md` (see SKILL.md, *Improvement log*); don't change the loop's own files.
