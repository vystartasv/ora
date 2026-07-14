# ORA — Universal Task Orchestrator

**One tool to decompose, route, delegate, compress, reconcile.**

Drop it in. It works with any agent (Claude Code, Codex, Pi, Cursor, Cline, Hermes — any of them) or as a standalone CLI. When you give it a task, it:

1. **Decomposes** — breaks your task into independent, verifiable subtasks
2. **Routes** — maps each subtask to the cheapest adequate model
3. **Delegates** — spawns subagents or calls CLI agents
4. **Compresses** — applies caveman/ponytail/RTK token optimisation to everything
5. **Reconciles** — verifies, merges, and reports

## Quick start

```bash
# Install
go install github.com/vystartasv/ora/cmd/ora@latest

# Or build from source
git clone https://github.com/vystartasv/ora
cd ora && go build ./cmd/ora

# Use
ora "build a login system with JWT"
ora "design the database schema for a multi-tenant SaaS" --plan
ora "fix the race condition in the worker pool" --fast
ora "review the authentication flow" --agent codex
```

## How it works

```
┌──────────────────────────────────────────────┐
│              ora "build auth system"          │
└──────────────────────┬───────────────────────┘
                       ▼
┌──────────────────────────────────────────────┐
│  1. DECOMPOSE — LLM breaks task into parts   │
│     A. Research existing patterns  [cheap]   │
│     B. Implement User model        [mid]     │
│     C. Implement JWT handler       [mid]     │
│     D. Write tests                 [mid]     │
│     E. Security review             [flagship]│
└──────────────────────┬───────────────────────┘
                       ▼
┌──────────────────────────────────────────────┐
│  2. ROUTE — each subtask → cheapest model     │
│     A. → qwen2.5-coder:3b (local, free)      │
│     B. → deepseek-chat ($0.14/M)             │
│     C. → deepseek-chat                       │
│     D. → deepseek-chat                       │
│     E. → deepseek-reasoner ($1.10/M)         │
└──────────────────────┬───────────────────────┘
                       ▼
┌──────────────────────────────────────────────┐
│  3. EXECUTE — spawn, wait, collect            │
│     Parallel: A ──┐                          │
│     Serial:  B → C ─┬→ E                     │
│     Parallel:   D ──┘                        │
└──────────────────────┬───────────────────────┘
                       ▼
┌──────────────────────────────────────────────┐
│  4. COMPRESS — caveman + ponytail + RTK       │
│     - Remove filler from all prompts/outputs  │
│     - YAGNI ladder for code generation        │
│     - Group/deduplicate/truncate results      │
└──────────────────────┬───────────────────────┘
                       ▼
┌──────────────────────────────────────────────┐
│  5. RECONCILE — verify + merge + report       │
│     ✅ All 5 subtasks completed               │
│     🪙 Savings vs all-flagship per run        │
│     📄 Report saved to .ora-report.json       │
└──────────────────────────────────────────────┘
```

## Modes

| Mode | Flag | What it does |
|------|------|-------------|
| **balanced** (default) | — | Routes per task type |
| **fast/cheap** | `--fast` | All subtasks → cheapest model |
| **deep** | `--deep` | All subtasks → flagship model |
| **plan** | `--plan` | Decompose only, show plan, no execution |

## For agents (not humans)

Drop `ORA.md` into any agent that reads rules files. It teaches the agent the same decomposition → routing → compression workflow. Works with Claude Code (`CLAUDE.md`), Cursor (`.cursor/rules/`), Cline (`.clinerules/`), Copilot (`.github/copilot-instructions.md`), Hermes (skills), and any other agent.

## Architecture

```
ora/
├── cmd/ora/main.go    # CLI entry point (flags: --plan, --fast, --deep, --mcp, --serve)
├── ora.go             # Core types, routing, model detection, config
├── decompose.go       # LLM-based task decomposition (oMLX → Ollama → API)
├── orchestrate.go     # Pipeline orchestration: decompose → route → execute → reconcile
├── mcp.go             # MCP server (stdio + HTTP, provides ora_decompose/route/execute tools)
├── ORA.md             # Universal agent prompt
├── CLAUDE.md          # Claude Code instructions
├── README.md          # This file
└── go.mod             # module github.com/vystartasv/ora
```

Pure Go. The MCP server is implemented in Go via `mcp.go`: stdio mode for Claude Code/Cursor and HTTP mode for Hermes.

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `ORA_API_KEY` | auto-detect | API key (fallback: DEEPSEEK_API_KEY, OPENAI_API_KEY) |
| `ORA_API_BASE` | `https://api.deepseek.com/v1` | API base URL |
| `ORA_API_MODEL` | `deepseek-v4-flash` | Model override |
| `ORA_MODE` | `balanced` | Mode: fast, balanced, deep |
| `ORA_AGENT` | `auto` | Agent: hermes, claude, pi, codex |

Auto-detects `DEEPSEEK_API_KEY` and `OPENAI_API_KEY` as fallbacks.

## Cost model

ORA estimates cost savings based on the **cost factor** assigned to each model tier:

| Tier | Cost factor | Used for |
|------|-------------|---------|
| cheap | 1 | lookup, research |
| mid | 2 | code_gen, review, plan |
| flagship | 10 | debug, architecture |

Savings are calculated per run as `100% − (actual cost / all-flagship cost × 100)`. The `.ora-report.json` saved after each run reports the exact breakdown — subtasks, models used, and the savings percentage for that specific task. Any stranger can reproduce: run `ora --plan "a task"`, read the routing in the plan, apply the cost factors above, and confirm the savings figure in the report.

## License

MIT — see [LICENSE](LICENSE)
