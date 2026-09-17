---
name: dont-let-your-token-die
description: Use when the user wants to stop wasting Codex tokens, distill past sessions into reusable project know-how, or turn local session-top digests into a skill draft. Not for live quota percentages or official billing.
---

# Don't let your token die

Orchestrate **project-scoped distillation**. `session-top` is a **dependency** (the engine). This skill only clarifies scope, runs the CLI, and helps draft a **future** skill. It does not recap chats for their own sake.

Success: a short skill a later session can load (do / don't, repo facts, corrections) so the next run spends fewer tokens. Not a weekly summary.

## Hard rules

1. Treat **session-top as a dependency**. Do not re-parse `~/.codex/sessions/**/rollout-*.jsonl` yourself.
2. **Clarify project (`cwd`), then time range, then content** with the user **before** extracting. Default project is the current working directory.
3. Never feed **raw rollout** JSONL (no `cat`, no dumping compacted payloads) into context. Read only `session-top distill` digest output.
4. Do **not auto-install** a generated skill. Present a draft; write a file only after the user names a destination.
5. Do not claim a quota percent drop was caused by missing knowledge. Distill is OBSERVED know-how, not OFFICIAL quota math.
6. `session-top`, `why`, and autopsy stay model-free. This skill may use a model only to turn an already-bounded digest into a SKILL.md draft after the user opts in.

## Check the CLI

```bash
session-top distill --help
```

If `session-top` is missing, tell the user to build it (`go build -o session-top ./cmd/session-top` from the session-top repo, or `make`) and stop. Do not invent a second parser.

## Workflow

### 1. Clarify (required)

Ask anything not already stated:

- **Project (`cwd`)**: current repo vs a path? Default: `.` resolved to an absolute path.
- **Time range**: `--since 7d` or `--from YYYY-MM-DD --to YYYY-MM-DD`. Time is secondary to project.
- **Content**: corrections / repo facts / things not to do again.

Do not run distill against all projects. Sessions whose cwd does not match must not mix into this project's digest.

### 2. Extract (CLI only)

```bash
session-top distill --cwd <project> --since 7d
session-top distill --cwd <project> --from 2026-09-01 --to 2026-09-17 --json
```

Use `--json` when you will draft a skill so you can cite `session_id` values. Summarize the digest for the user (goals, tools, mix, continuation counts, parent/fork ids). Ask what to keep.

### 3. Draft (only if asked)

If the user wants a reusable skill, write a **SKILL.md draft** in the response first:

- `name` hyphen-case for the **project** skill (not `dont-let-your-token-die`)
- `description` with triggers for that repo
- do / don't / repo facts
- cited session ids from the digest
- `model-written: true` in the body if a model produced the prose

Keep it short. No transcript paste.

### 4. Install (never default)

Offer destinations; wait:

- Repo: `.codex/skills/<project-skill>/SKILL.md`
- User: `~/.codex/skills/<project-skill>/SKILL.md`

Do not copy until they confirm. Do not overwrite without asking.
