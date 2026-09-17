# Project-scoped session distillation (design)

session-top already answers *where quota and tokens went* (overview, sessions, autopsy, why). Distillation is a **side path** that answers a different question: *what should the next session in this repo already know, so it does not pay tuition again?*

This document is the design and the **value goals**. It does not implement `session-top distill` and it does not call a model.

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

- Implementing `session-top distill` in this document's PR sequence is future work; this file is the contract.
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

`session-top distill` (future command) only:

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

Not implemented in this change. When implementing:

1. **Extract CLI** — `session-top distill` writes extracts + digest only (no model). Tests on testdata cwd + time window.
2. **Filter quality** — continuation/fork drop + caps; tests on fixtures with 继续 and `forked_from_id`.
3. **Opt-in draft writer** — separate package; skipped unless flag set; golden test on a tiny digest → SKILL.md.draft shape (no live LLM required if the writer is injectable).
4. **Docs / README** — link this design; restate that autopsy/why stay local.
