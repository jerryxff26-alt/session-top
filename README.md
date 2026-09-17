# session-top

htop for your agent sessions.

You know your usage dropped 12%.
Now find out why.

```
5h      ██████░░░░ 61%
Weekly  ████████░░ 84%

🔥 Top quota consumers

OAuth bug        8.2%
Database         3.1%
README           0.3%
```

```
brew install session-top
session-top
```

session-top makes coding-agent usage observable and explainable.

v0.1 reads **Codex CLI** rollouts only. The name is about sessions, not a single vendor.

## Install

From source (Go 1.23+):

```
git clone https://github.com/jerryxff26-alt/session-top.git
cd session-top
make
./session-top
```

Or `go build -o session-top ./cmd/session-top`. Homebrew (`brew install session-top`) is the intended distribution line; this repo is the source.

## Repository layout

```
cmd/session-top                         CLI entrypoint
internal/cli                            command dispatch
internal/codex                          discover and parse ~/.codex rollout JSONL
internal/usage                          OFFICIAL / OBSERVED / INFERRED analysis
internal/tui                            terminal rendering
testdata                                fixture Codex home for tests
docs/distill.md                         distillation design
skills/dont-let-your-token-die          orchestrator skill (copy into Codex/Grok)
```

## Usage

```
session-top              # 5h / weekly overview, today, top consumers
session-top why          # potential causes for a recent quota drop
session-top sessions     # rank sessions by inferred quota Δ
session-top session <id> # turn timeline for one session
session-top watch        # live-refreshing view
session-top distill      # project digest (cwd + time; no model)
```

Codex data is read from `~/.codex/sessions/**/rollout-*.jsonl`. Override the Codex home directory with `CODEX_HOME`.

## Skill: dont-let-your-token-die

English name for “don't let your token die”. It is an **orchestrator**, not a dump of one project's chats.

- **CLI is the engine** (`session-top distill`): 100% local, no model, bounded digest (goals, corrections, tools, observed mix, session ids).
- **Skill is the conversation**: clarify **project (`cwd`)**, then **time range**, then **what to keep**, then run the CLI. Never `cat` raw rollout JSONL. Never auto-install a generated skill.
- **Value**: a short skill a *later* session can load (do / don't, repo facts), so the next run spends fewer tokens — not a weekly recap.

### Install

The repo keeps **one** copy: `skills/dont-let-your-token-die/`. Copy it into the agent you use:

```
# Codex (this repo)
mkdir -p .codex/skills
cp -R skills/dont-let-your-token-die .codex/skills/

# Codex (user-wide)
mkdir -p ~/.codex/skills
cp -R skills/dont-let-your-token-die ~/.codex/skills/

# Grok Build (user-wide)
mkdir -p ~/.grok/skills
cp -R skills/dont-let-your-token-die ~/.grok/skills/
```

Invoke: Codex `$dont-let-your-token-die` · Grok `/dont-let-your-token-die`

`session-top` must be on `PATH` (or `make` in this repo). Distill itself does not call a model.

### Example

```
session-top distill --cwd /path/to/project --from 2026-09-16 --to 2026-09-16
```

A product skill for that project is a **draft you copy by hand** after review. session-top will not write into `~/.codex/skills` or `~/.grok/skills` for you.

## What the numbers mean

OpenAI does not publish a formula that maps tokens × model × reasoning × cache × tools onto quota percent. session-top never invents one.

| Class | Meaning |
| --- | --- |
| **OFFICIAL** | Quota % and reset times the agent stored from the provider (`rate_limits`) |
| **OBSERVED** | Tokens, turns, models, tool calls, compactions from local session events |
| **INFERRED** | Quota Δ between consecutive official snapshots, attributed only when a single session was active |

If two sessions overlap an interval, that Δ is **Attribution: ambiguous** — listed, not guessed.

Some Codex modes (historically `codex exec`) record `rate_limits: null`. OBSERVED still works; OFFICIAL / INFERRED quota is omitted or marked unknown.

## session-top does NOT

- bypass Codex (or other agent) limits
- predict a provider's private quota formula
- intercept prompts
- upload conversations
- send telemetry
- require OpenAI API keys

**100% local.**

## Scope (v0.1)

Supports Codex CLI rollouts on macOS and Linux.

Not in v0.1: other agents, Windows, Codex Desktop / VS Code / Cursor, roast reports, JSON export, anomaly detection, multi-machine, historical trends.

## Design

Project-scoped distillation: [docs/distill.md](docs/distill.md). Extract CLI and orchestrator skill are in-tree; autopsy/why stay 100% local.

## License

MIT
