---
name: loop-coding
description: "Supervisor/worker loop for coding work: one worker sub-agent per task (model tier chosen per task), the supervisor verifies the real result itself (tests/build/diff, never the worker's self-report), re-prompts with concrete feedback, escalates the tier when stuck, and stops when a checkable definition-of-done holds. Works for a single task or a whole project driven by README.md → Plan.md → Tasks.md. Use when the user asks to delegate or supervise work, build a project from a spec, run Tasks.md, or invokes /loop-coding."
---

# Supervise loop

The supervisor (this session) briefs, verifies, records and commits. It never writes product
code itself; workers do. Args: a task description, or `Tasks.md`, or nothing (= project mode).

Terms. **Round**: one worker attempt (first dispatch or one feedback message) plus your
verification. **Protected paths**: everything the spec's *Constraints* marks untouchable, plus
`README.md`, `Plan.md`, `Tasks.md`, `tasks/`, `scripts/verify/`, `scripts/check-protected.sh`,
`scripts/protected.sha256`, `Improvements.md`, `prompts/`, `.claude/` and the "Delegation
workflow" section of `CLAUDE.md`. Workers never edit protected paths (exception: the generator
workers of step 0 write `Plan.md`, `Tasks.md`, `tasks/` and `scripts/verify/`); only you edit
`Tasks.md` (status and notes) and `Improvements.md`. Documentation therefore goes under `docs/`, never into `README.md`.
**Task files**: `Tasks.md` is only a small index (ID, phase, title, dependencies, tier, status,
notes) plus the *Standing constraints*; the full brief of a task is `tasks/<ID>.md` and its
verification is the executable `scripts/verify/<ID>.sh` (exit 0 = pass). Read the index every
iteration, read a task file only when you dispatch or repair that task, never the whole `tasks/`
directory.
**Check command**: the project-wide command from Plan.md *Commands* (tests + lint + build); in
single-task mode the project's own test command (see `CLAUDE.md`, else discover it).

## 0. Choose the mode

- **Single task** (bug, feature, refactor): go to step 1 with that task.
- **Project** (args `Tasks.md`, empty, or "build the project"; spec in `README.md`):
  1. `README.md` missing or `grep -q SPEC-PLACEHOLDER README.md` succeeds → stop and tell the
     user to create the spec first (the placeholder contains the prompt).
  2. No `Plan.md` → dispatch one `opus` worker: "Follow `prompts/generate-plan.md` with
     `README.md` as the spec; write `./Plan.md`." Verify yourself: all eight sections exist
     (`grep -cE '^## (Scope|Architecture|Commands|Assumptions Log|Build Phases|Global Definition of Done|Model-Routing Policy|Risks)' Plan.md`
     prints 8), Phase 0 Preflight exists, `grep -inE 'TBD|ask the user' Plan.md` is empty,
     every acceptance criterion maps to a phase or DoD line (read, don't grep). Commit `Plan.md: v1`.
  3. No `Tasks.md` → one `opus` worker with `prompts/generate-tasks.md` (input `Plan.md` +
     `README.md`); with more than ~15 tasks to generate, one dispatch per phase batch, each
     committed (`Tasks.md: batch n`). Verify: every phase has tasks, every row has a tier and a
     `tasks/<ID>.md`, every worker row has an executable `scripts/verify/<ID>.sh` that passes
     `bash -n` and contains a negative check for opus rows, the index is well-formed
     (`awk -F'|' '/^\| T/{print NF}' Tasks.md | sort -u` prints exactly one number; no pipes
     inside cells), and every README requirement R-number appears in a task file or in Plan.md's
     coverage table. Commit `Tasks.md: initial`.
  4. **Checkpoint:** show the user the Consequential assumptions and tasks per tier; continue
     after their OK. Skip only if the user said to run unattended.
  5. Run `Tasks.md`: do step 1 once (it is row `T000`, `Model` = `supervisor`: you run its
     verification yourself, no worker; then mark it `done` and commit) and step 3 once (arm
     `/goal`). Then repeat steps 4–6 per row: always the first `todo` row whose `Depends-on` are
     all `done`, one row at a time. Step 2 is already the row's verification script.
     `Tasks.md` is the state: after a restart, continue from its statuses.

## Resume

On every start, run `git status --porcelain` and sort the dirty paths:
- **Protected paths** (`README.md`, `Plan.md`, `Tasks.md`, `tasks/`, `prompts/`, …) are edits by the user or
  by you, never an orphan: commit them first as `<file>: update`. `Improvements.md` is the
  exception: leave it, it goes into the next commit.
- **All other paths** are the leftover of an interrupted task. With `Tasks.md`: run the first
  selectable `todo` row's `scripts/verify/<ID>.sh`. Passes → treat it as a worker result (step 5). Fails →
  `git stash push -u -m "orphan <ID>" -- <those paths>` (never the protected ones), note it in
  the row, continue. Without `Tasks.md` (single-task mode, or before planning) these are the
  user's own changes: stop and ask them to commit or stash.

Never start a worker on a dirty tree (apart from `Improvements.md`).

## 1. Preflight

Run the toolchain checks yourself (Plan.md Phase 0, or the obvious ones: runtime, package
manager, required external tools, env vars, `git status`). If something is missing, stop and give
the user the exact install command; workers can't fix a missing toolchain and burn tokens trying.

## 2. Define done

Exact pass/fail commands (tests, build, lint, a request + expected output), never "looks right".
Each must terminate on its own (`timeout`, no foreground servers). Where practical, see it fail
before the work starts, so a pass means something. If it's ambiguous and nothing in the repo or
conversation resolves it, ask once; otherwise decide and state the assumption in the final report.

## 3. Keep the loop alive

Arm `/goal` with the done condition (project mode: "every row in Tasks.md is done, superseded or
blocked-with-nothing-selectable, and the Global Definition of Done passes"). It keeps the session
working across turns; stop it with `/goal clear`. Don't build your own retry logic on top.

## 4. Dispatch one worker

`Agent` with `subagent_type: general-purpose` (never `fork`), a clear `description`, and `model`:
- `haiku`: small, bounded, mechanically checkable
- `sonnet`: default
- `opus`: architecture; security-sensitive code (auth, secrets, untrusted input, shell/SQL
  construction); concurrency; anything tied to a Consequential assumption

The brief is self-contained (the worker has no memory of this conversation): the *Standing
constraints* from `Tasks.md` (prepend them verbatim), then the *Brief* of `tasks/<ID>.md`
(goal, files to read and change, interfaces), the verification command
`scripts/verify/<ID>.sh`, and only the Plan/spec excerpts it needs, not whole documents. It also states: use only the live systems the
spec's Environment allows, never any other host; never write secrets into files; install nothing
globally (report a missing tool instead). End every brief with:
"Run the verification yourself before reporting. Do not commit. Report in at most 10 lines:
files changed, commands run, result. If the process itself got in your way (unclear brief,
wrong verification, missing context), add one line `Loop note: <what and how to fix it>`."

## 5. Verify yourself

- `git status` and `git diff --stat`; read only the hunks that matter.
- Scope: `git diff --name-only` contains no protected path (except your own `Improvements.md`
  entry) and nothing outside the task; no tests
  deleted, skipped or weakened to get green; no secrets or `.env` files in the diff
  (`git diff | grep -niE 'secret|token|passw|api[_-]?key'`, read the hits).
- Run `scripts/verify/<ID>.sh` yourself (it prints one line per check and the tail of a failing
  one) and `scripts/check-protected.sh` (protected files and the Delegation section unchanged).
- Check point by point, including negative cases for opus tasks.
- Regression: run the check command, so a pass doesn't break earlier work. If it takes more than
  a couple of minutes, run it at phase ends and before the Global Definition of Done instead.

## 6. Pass, fail, stuck

- **Pass:** one commit with the task's files and the `Tasks.md` update (status `done`, note:
  files + one line; no hash, it isn't known yet). Message `<task id or short title>: <summary>`.
  Next task.
- **Fail:** `SendMessage` to the same worker: the failing command, the relevant output lines,
  what to change. Never just "try again".
- **Stuck after 3 rounds with the same worker:** escalate one tier (haiku → sonnet → opus); at
  `opus`, start a fresh opus worker instead. Rounds count per task, not per worker, so a `haiku`
  task reaches `opus` with only 2 rounds left. A worker's model can't be changed, so spawn a new
  worker with a handoff:
  task, what was tried, the failing output, the current diff state. You decide whether the new
  worker continues from the diff or from scratch; in the latter case revert only this task's
  files (`git restore <files>`), never `git reset --hard`, and say so in the handoff. Note the
  escalation in the row.
- **Broken row** (the verification script itself is wrong or can't terminate, or the task
  contradicts the spec): fix `tasks/<ID>.md` or `scripts/verify/<ID>.sh`, note `row fixed: <why>`
  in the index; this isn't a round. If fixing
  means deviating from the spec or a Consequential assumption → `blocked`.
- **Budget:** at most 8 rounds per task in total. Out of rounds → set `blocked` with what blocks
  it and what was tried, then continue with every row that doesn't depend on a blocked one.
  Note the rounds used in the row at every escalation and at `blocked`, so the count survives a
  restart; a row the user sets back to `todo` gets a fresh budget. When nothing selectable
  remains and something is blocked → stop and report. During task execution this is the only
  point where the loop waits for the user (besides the checkpoint, preflight and a missing spec).
- **Project done:** nothing selectable and nothing blocked → run the Global Definition of Done.
  A failing item becomes a new fix row (`todo`, tier by the usual rules; new `tasks/<ID>.md` and
  `scripts/verify/<ID>.sh` = that failing check) and the loop continues. After 2 such repair passes, stop and report the rest.
  Final report: what was built, the assumptions made, the commit range, the DoD output.

## Improvement log

The loop gets better only if its friction is written down. Whenever you notice a problem with the
**loop itself** (this skill, `prompts/generate-*.md`, the `README.md` spec template, the
`Tasks.md` format, the escalation rules, a recurring failure pattern, a worker's `Loop note`),
append one row to the table in `Improvements.md` in the project root (it starts as an empty
template with the header below; recreate it if missing). Not for defects in the product code:
those are tasks.

```
# Improvements
| Date | Source | Target | Observation | Proposal |
|---|---|---|---|---|
```

- **Source**: task ID or step (e.g. `T012, step 6`). **Target**: file to change (`SKILL.md`,
  `generate-plan.md`, `generate-tasks.md`, `README.md` template, `CLAUDE.md` template).
- **Observation**: what happened, with evidence (one line). **Proposal**: the concrete change
  (one line, ideally the new wording). No pipe characters inside a cell.
- Only recurring or costly friction, not one-off typos. Check first that the same observation
  isn't already listed.
- Never apply a proposal to the loop's own files yourself (they are protected); only log it.
  Include `Improvements.md` in the commit of the task that triggered the entry; with no task
  commit pending (preflight, final check), commit it alone as `Improvements.md: <short title>`.
  Mention the new entries in the final report.
- This costs no round and never blocks a task.

## Token rules

- Worker transcripts stay out of this session: read the final report, verify with targeted
  commands, never read whole transcripts or dump large files. Per iteration read the `Tasks.md`
  index and one task file, nothing more.
- One task per worker; the right tier per task (quality first, but no opus for mechanical work).
- Workers run one after another, never in parallel: the scope check via `git diff` and one commit
  per task rely on a tree that only one worker touches.
