# Improvements

Friction in the loop itself (skill, prompts, spec template), logged by the supervisor. Rules:
`.claude/skills/loop-coding/SKILL.md`, section *Improvement log*. Never edit by hand while the
loop runs; take entries over into the vault note afterwards.

| Date | Source | Target | Observation | Proposal |
|---|---|---|---|---|
| 2026-10-08 | step 0.2, Plan worker | README.md template | Spec tool table and install line omit x11-xcb (libx11-xcb-dev, needed by Gio) and the spec cites assumptions A1-A5 that it never defines (A3 missing); the Plan numbers its own assumptions differently | Template Environment section: list the full pkg-config set Gio needs; either define A1-An in the spec or forbid references to undefined assumption IDs |
