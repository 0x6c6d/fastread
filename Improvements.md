# Improvements

Friction in the loop itself (skill, prompts, spec template), logged by the supervisor. Rules:
`.claude/skills/loop-coding/SKILL.md`, section *Improvement log*. Never edit by hand while the
loop runs; take entries over into the vault note afterwards.

| Date | Source | Target | Observation | Proposal |
|---|---|---|---|---|
| 2026-10-08 | step 0.2, Plan worker | README.md template | Spec tool table and install line omit x11-xcb (libx11-xcb-dev, needed by Gio) and the spec cites assumptions A1-A5 that it never defines (A3 missing); the Plan numbers its own assumptions differently | Template Environment section: list the full pkg-config set Gio needs; either define A1-An in the spec or forbid references to undefined assumption IDs |
| 2026-10-08 | step 0.3, Tasks batch 1 | generate-tasks.md | Consequential-implies-opus rule turned small tasks that only add one pinned dependency (ORP, LICENSE) into opus although the Plan examples list them lower | Add: a task that merely adds one pinned dependency via an exact command, checked by its script, may stay at its natural tier |
| 2026-10-08 | step 0.3, Tasks batch 2 | generate-tasks.md | Interrupted batch left committed-but-unindexed task drafts without the executable bit on scripts; RESUME section does not cover them; opus-for-Risk rule made 10 of 14 Phase 2 tasks opus | In Resuming: adopt drafts in tasks/ that have no index row, chmod +x all verify scripts; allow sonnet for test-only tasks whose code is written by an earlier opus task
| 2026-10-08 | step 0.3, Tasks batch 4 | generate-plan.md, T000 | Plan v1 chose font8x8 (not available offline); batch 4 had to amend Plan.md to v1.1 using system X11 misc-fixed fonts, and Phase 0 preflight did not check the font files | generate-plan.md: verify every external asset the Plan names is obtainable offline; add its presence to Phase 0 preflight
