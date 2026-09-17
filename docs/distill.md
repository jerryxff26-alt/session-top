# Project-scoped session distillation (design)

session-top already answers *where quota and tokens went* (overview, sessions, autopsy, why). Distillation is a **side path** that answers a different question: *what should the next session in this repo already know, so it does not pay tuition again?*

This document is the design and the **value goals**. The extract CLI (`session-top distill`) is model-free. The orchestrator skill **dont-let-your-token-die** depends on that CLI. Generating a *product* skill with a live LLM is still opt-in and out of the default core.

## Value goals

Success is **not** a recap of chats, a weekly diary, or a summary of the most expensive session.

Success is a **skill a later session can load**:

- Capture reusable **project know-how**: do / don't, repo facts, user corrections, working paths.
- Reduce **future** agent spend (fewer rediscovery turns, less re-sent context, fewer “继续” loops on the same task).
- Stay honest about data classes: autopsy/why remain OFFICIAL / OBSERVED / INFERRED and **local**. Distillation output is a **model-written draft**, never an official quota explanation.

A draft that nobody loads is a failed distill, even if the prose is good.

Concrete value in this user's real rollouts (illustrative, not a formula):

| Observed waste | Distill target |
| --- | --- |
| Same LVMH report `/goal` appearing as many high-token sessions | Skill: increment the existing report structure; do not regenerate from scratch |
| “继续” threads at tens of millions of observed tokens | Skill: stop and ask for a new goal instead of extending a bloated thread |
| Rediscovering that SearXNG / OrbStack is already running | Skill: repo fact — local stack is already up |
| Repeated visual corrections (“don't change the architecture icons”) | Skill: don't / do list from user corrections |

If those facts are in a loaded skill, the next session should spend less OBSERVED input and fewer turns. That is the only value target that justifies an optional model call.

## Non-goals

- Default-on model calls. Distill is **opt-in**, never part of `session-top`, `why`, `watch`, or autopsy.
- Auto-enabling a skill without **human review**. No write into Codex/Grok skill directories unless the user later copies a reviewed draft.
- Feeding **raw rollout** JSONL (or full transcripts) to a model.
- A “distill everything” default across all projects.
- Semantic summaries of the most expensive session as the primary artifact.
- Claiming a quota % drop “was caused by” missing knowledge or by distillation topics.
- Changing autopsy/why to require a network or an API key.

## CLI vs skill split

Two products, one data plane. Do not embed a chatbot in session-top.

### CLI (session-top) — extract, no model required

`session-top distill` only:

1. Selects sessions by **project (`cwd`)** then **time range**.
2. Filters noise (continuation-only prompts, fork replays of a parent, empty goals).
3. Emits **bounded extracts** per session and a **project digest** (merge/dedup).
4. Writes local JSON/markdown extracts under a user-chosen path.

This stage is 100% local, same as autopsy/why. It does not need an OpenAI API key and does not upload conversations.

### Skill draft — opt-in model, human review

Only if the user opts in (`--write-skill` or piping the digest to a model they control):

1. The model sees the **project digest**, not raw rollouts.
2. It writes a **SKILL.md draft** marked `model-written`.
3. The draft **cites session ids** so a human can run `session-top session <id>` to verify.
4. **Human review** before enable. session-top never auto-installs the file into an agent skill directory.

Autopsy/why stay local. Distill is an explicit side path, not the default 100%-local core.

There is no default that calls a model when the user runs `session-top` with no subcommand.

## Orchestrator skill: dont-let-your-token-die

English name (user: “dont let your token die”): **dont-let-your-token-die**.

The CLI can run alone. Inside Codex, the skill is the **orchestrator** and session-top is a **dependency**:

1. Check `session-top` is on `PATH`.
2. Clarify **project (`cwd`)**, then **time range**, then **content** (corrections / repo facts / don'ts) before extracting.
3. Exec `session-top distill --cwd … --since …` (optional `--json`). Read **only the digest**.
4. Never `cat` raw rollout JSONL. Never auto-install a generated skill.

Shipped path: `skills/dont-let-your-token-die/SKILL.md` (copy into `.codex/skills`, `~/.codex/skills`, or `~/.grok/skills`).

### OSS review (borrow patterns, do not clone)

| Source | Takeaway | We do / don't |
| --- | --- | --- |
| [Codex custom skills](https://developers.openai.com/codex/skills/create-skill) | Progressive disclosure; `SKILL.md` + optional `scripts/`; repo vs user scope | Instruction-only skill; exec CLI rather than re-parse JSONL |
| [gouzigouzi/codex-local-token-usage-skills](https://github.com/gouzigouzi/codex-local-token-usage-skills) | CLI is the core; skill only says when to run which command | Same split |
| [entireio session-to-skill](https://github.com/entireio/skills/blob/main/skills/session-to-skill/SKILL.md) | Ask the reusable behavior **before** reading transcripts; default present draft, write only after destination | Clarify cwd/time/content first; no auto-install |
| [skill-distill](https://www.npmjs.com/package/skill-distill) | CLI from sessions; `--install` exists | **Reject `--install` as default** (and do not ship it) |
| [c-daly/agent-swarm distill](https://agent-skills.md/skills/c-daly/agent-swarm/distill) | Bucket pattern / pitfall / preference | Digest fields: goal, corrections (pitfalls), tools/files (approach) |
| [lokikill123/codex-token-skills](https://github.com/lokikill123/codex-token-skills) | Freeze short skills; don't dump huge instructions every turn | Keep orchestrator SKILL.md short; product skills stay small |
| [Redclawww/savethetokens](https://github.com/Redclawww/savethetokens) | Hygiene *during* a live session | Different problem; we distill *past* sessions into future know-how |

## Invocation (project default, time secondary)

Project scope is the **default dimension**. Time is a filter, not the grouping key.

```
session-top distill
session-top distill --project .
session-top distill --cwd /Users/me/git-repo/elc --since 7d
session-top distill --cwd /Users/me/git-repo/elc --from 2026-09-01 --to 2026-09-17
```

Rules:

- Default `--cwd` is the current working directory (the repo the user is in).
- A session is in-scope when `session_meta.cwd` equals that project or is inside it.
- Sessions whose `cwd` does not match **are not mixed** into that project's skill (including an `unknown` cwd bucket).
- `--since` / `--from` / `--to` bound timestamps; omitting time means “all retained rollouts for this project” still **project-scoped**, never global.
- There is no default command that distills all projects at once.

Output paths (illustrative):

```
./.session-top/distill/<project-slug>/extracts.jsonl
./.session-top/distill/<project-slug>/digest.md
./.session-top/distill/<project-slug>/SKILL.md.draft
```

Drafts stay in the project (or a user `--out` dir) until a human moves them.

## Bounded extracts (no raw rollout dump)

Each in-scope session becomes one extract object with hard caps. Overflow is truncated here, not sent as JSONL.

| Field | Source (already on disk) | Cap (design) |
| --- | --- | --- |
| `session_id` | `session_meta.id` | full |
| `cwd` | `session_meta.cwd` | full |
| `parent_id` | `forked_from_id` if present | full |
| `goal` | first non-noise user prompt | ~500 chars |
| `corrections` | later user prompts that constrain work (“不要…”, “谁让你…”, “don't…”) | max 8, ~240 chars each |
| `tools` | function_call names + counts | top 8 names |
| `files` | paths from tool args / patch ends, basenames preferred | top 12 |
| `observed` | input / cached share of input / output / reasoning / turns / compactions | numbers only |
| `continuation_follow_ups` | keep-going / 继续 style prompts after the first | count |
| `title` | same title rules as `sessions` | ~80 chars |

Not included: full assistant text, full tool stdout, compacted payloads, environment_context dumps.

Per-session extract budget: **target ≤ 2k tokens, hard stop 4k**. If still over: drop files, then tools, then extra corrections; never expand to the rollout file.

## Project digest, merge, shard

After extracts:

1. **Drop** extracts with no `goal` and only continuation follow-ups.
2. **Dedup** forks: children of the same `parent_id` contribute corrections/files but not a second copy of the parent's goal; do not rewrite OBSERVED totals (same rule as v0.1).
3. **Merge** repeated goals (similar title/goal) into one digest item with `repeat_count` and listed session ids.
4. **Compose** a project digest: do/don't from corrections, repo facts from repeated files/cwd, hot paths, observed shape (token vs quota rank if known).

Digest budget: **target ≤ 8k tokens, hard stop 16k** of model-visible text.

If still over-budget: **split-then-compose**

- Shard by week or by merged-goal cluster.
- Each shard produces a skill *section* (still a draft fragment).
- A final compose pass sees only those sections, not extracts.

Over-budget input is truncated or sharded. Full JSONL is never the model prompt.

## Skill-draft contract

When (and only when) the user opts into a model:

The draft MUST:

- Be a `SKILL.md` (or `.draft`) with YAML front matter: `name`, `description`, `version`.
- State `model-written: true` and the digest time range + cwd.
- List **cited session ids** (and not claim facts that lack a citation).
- Encode **triggers** (this repo / this task type), **do**, **don't**, **repo facts**.
- Stay short enough to load as a skill (aim < 2k tokens of instruction).

The draft MUST NOT:

- Be auto-enabled or copied into `~/.codex/skills` / `~/.grok/skills` by session-top.
- Assert that a quota percent “was caused by” missing a skill.
- Include raw prompts beyond the short `goal` / `corrections` already in the digest.
- Run unless the user passed an explicit write/opt-in flag.

Human review is mandatory: the user reads the draft, edits, then installs it themselves (or discards it).

## Data flow

```
~/.codex/sessions/**/rollout-*.jsonl
        │  cwd matches --project / --cwd
        │  timestamp in time range
        │  drop continuation-only and parent replays
        ▼
bounded per-session extracts     (local, capped)
        │
        ▼
project digest (dedup, corrections, hot files)
        │  still large → shard by week/topic, then compose
        ▼
optional opt-in model → SKILL.md.draft (model-written, cited ids)
        │
        ▼
human review → enable by hand or throw away
```

`session-top session <id>` remains the way to inspect a citation. Distill never replaces autopsy.

## Risks

- **Quota spend on distill**: an opt-in model call costs the resource we are trying to save. Mitigate with tiny digests and no default-on calls.
- **Garbage in**: “继续” and forks dominate real traces. Filter before extract or the skill will teach the agent to ramble.
- **Wrong project mix**: global distill would blend unrelated repos. cwd default prevents that.
- **Stale skills**: a draft is a snapshot; it is not live autopsy. Version and date the skill; users re-run distill when the project changes.
- **Privacy**: extracts stay local; opt-in model is the user's model/CLI, not session-top telemetry.

## Key decisions

1. **Value = future loaded skill, not a recap** — otherwise distill competes with `why` and burns tokens for a document nobody uses.
2. **CLI extract is model-free; generate is opt-in** — preserves 100% local core (autopsy/why).
3. **cwd / project is the default dimension; time range is secondary** — matches how people actually run sessions.
4. **Bounded extracts + shard, never raw JSONL** — context overflow is an extract bug, not a reason to buy a bigger window.
5. **Human review, cited session ids, no auto-enable** — skills change future agent behavior; silent install is unsafe.
6. **Do not mix unmatched cwd into the project skill** — including unknown cwd.

## Open questions (implementation later, not blockers for this design)

- Which opt-in mechanism (flag vs user-run editor vs piping digest to `codex exec`) — all are explicit; none are default-on.
- Exact similarity threshold for merging goals — implementation detail; digest must still cite ids.

## PR Plan

1. **Extract CLI (this change)** — `session-top distill` writes a bounded digest (no model). Tests on testdata `--cwd /workspace/oauth-app`.
2. **Orchestrator skill (this change)** — `dont-let-your-token-die` depends on the CLI; stepwise cwd/time/content; no raw JSONL; no auto-install.
3. **Opt-in draft writer** — later; skipped unless the user asks the skill to draft after seeing the digest.
4. **Docs / README** — this file + README usage line.
