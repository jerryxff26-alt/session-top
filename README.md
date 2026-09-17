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
cmd/session-top    CLI entrypoint
internal/cli       command dispatch (overview, why, sessions, watch)
internal/codex     discover and parse ~/.codex rollout JSONL
internal/usage     OFFICIAL / OBSERVED / INFERRED analysis
internal/tui       terminal rendering
testdata           fixture Codex home for tests
docs/distill.md    design: project-scoped distillation → skill draft
```

## Usage

```
session-top              # 5h / weekly overview, today, top consumers
session-top why          # potential causes for a recent quota drop
session-top sessions     # rank sessions by inferred quota Δ
session-top session <id> # turn timeline for one session
session-top watch        # live-refreshing view
```

Codex data is read from `~/.codex/sessions/**/rollout-*.jsonl`. Override the Codex home directory with `CODEX_HOME`.

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

Project-scoped distillation into a reviewable skill draft (not implemented in v0.0.1; autopsy/why stay 100% local):

[docs/distill.md](docs/distill.md)

## License

MIT
