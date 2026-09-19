---
name: dont-let-your-token-die
description: Use when the user wants to stop wasting Codex tokens, rank past project sessions with Jev, or distill bounded Codex history into reusable repo guidance. Not for live quota percentages or automatic deletion.
---

# Don't let your token die

Turn project-scoped Codex history into a short skill a later session can load. `session-top` is the required extraction dependency; this skill orchestrates scope, optional Jev ranking, drafting, and review.

Success is reusable **do / don't / repo facts / verified procedures**, not a weekly chat recap.

## Invariants

1. Scope by **project (`cwd`) first**, then **time range** or `--session ID`. Never mix unmatched projects.
2. Never read or paste a raw rollout JSONL into model context. Use only the bounded, redacted `session-top distill` digest.
3. The local digest must include user messages, assistant conclusions, tool evidence, and `context_coverage`. If coverage is incomplete, do not treat low value as proven.
4. `--jev` is explicit opt-in. It sends only bounded, redacted digest context to TypeSafe; default distill stays local and model-free.
5. Jev ranks sessions; it does not write the final skill. Use its separate reusable-knowledge, verified-evidence, and correction-value judgments as signals.
6. A low Jev label alone does **not** authorize archive or deletion. Archive only through the complete safety gate below, and only after explicit `--apply`.
7. Do **not auto-install** a generated skill. Draft first; write only after the user chooses a destination. Never overwrite without approval.
8. Archive is reversible (`codex unarchive <SESSION>`); deletion is never part of this skill.

## Check the dependency

```bash
session-top distill --help
```

If unavailable, tell the user to build it from this repo (`make` or `go build -o session-top ./cmd/session-top`) and stop. Do not invent another rollout parser.

## Workflow

### 1. Resolve scope

Use what the user already supplied; ask only for missing scope:

- project path, defaulting to current `cwd`;
- time range (`--since`, `--from` / `--to`) or an exact/prefix/suffix session id;
- desired knowledge: corrections, repo facts, verified procedures, or all three.

### 2. Extract locally first

```bash
session-top distill --cwd <project> --since 7d --json
session-top distill --cwd <project> --session <id> --json
```

Inspect every candidate's:

- `context`: bounded user / assistant / tool evidence;
- `context_coverage`: included, omitted, truncated, oversized, complete;
- corrections, tools, goal, and session id.

Do not claim a conclusion was verified unless the context contains corresponding command/test evidence.

### 3. Rank with Jev only when requested

Requires `JEV_API_KEY` (or compatible `TYPESAFE_API_KEY`):

```bash
session-top distill --cwd <project> --session <id> --jev --json
session-top distill --cwd <project> --since 2d --jev --json
```

A Jev run is capped at 10 matched sessions; narrow the scope instead of bulk-uploading history. Treat `priority: review` or `review_required: true` as a mandatory manual/strong-model review. Never discard a session solely because Jev says `low` or `none`.

### 4. Optionally archive proven low-value sessions

Preview candidates first; this command does not change Codex state:

```bash
session-top distill --cwd <project> --since 30d --jev --archive-low --json
```

A session is a candidate only when every gate passes: context is complete, review is not required, it is at least 7 days old, it is not the current `CODEX_THREAD_ID`, Jev priority is `low` or `none`, and reusable-knowledge, verified-evidence, and correction-value scores are all below `0.35`. Everything else is marked `protected`.

After reviewing the candidate list, execute Codex-native archive only when the user explicitly asks:

```bash
session-top distill --cwd <project> --since 30d --jev --archive-low --apply --json
```

`--apply` archives candidates with `codex archive <SESSION>`. Archive is reversible with `codex unarchive <SESSION>` and is not deletion. Never add `--apply` implicitly, never archive the current task, and never use `codex delete`.

### 5. Draft the reusable project skill

Use only supported claims. The draft should contain:

- YAML front matter with a project-specific hyphen-case `name`, discriminating `description`, and `version`;
- `model-written: true`, source `cwd`, and digest time range;
- concise triggers, do, don't, repo facts, and verified procedures;
- cited session ids next to material claims;
- uncertainties or incomplete-context warnings.

Aim below 2k tokens. Do not paste transcript excerpts unless a short quote is necessary to explain a correction.

### 6. Human review and optional install

Present the draft in the response. If the user chooses to write it, offer:

- repo-local: `.codex/skills/<project-skill>/SKILL.md`
- user-wide: `~/.codex/skills/<project-skill>/SKILL.md`

Do not auto-install or delete anything. Archive only through the preview-first flow above and only with explicit `--apply`.
