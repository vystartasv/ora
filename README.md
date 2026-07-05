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
git clone https://github.com/yourname/ora ~/projects/ora
ln -sf ~/projects/ora/cli/ora ~/.local/bin/ora

# Use
ora "build a login system with JWT"
ora "refactor the API to use async handlers and add tests"
ora "design the database schema for a multi-tenant SaaS" --plan
ora "fix the race condition in the worker pool" --fast
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
│     🪙 68% cheaper than all-flagship          │
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
├── cli/
│   ├── ora          # Shell entry (bash wrapper)
│   └── ora.py       # Python implementation (the engine)
├── mcp/
│   └── server.py    # MCP server (for MCP-capable agents)
├── ORA.md           # Universal prompt (for any agent)
└── README.md        # This file
```

## Environment

| Variable | Default | Description |
|----------|---------|-------------|
| `ORA_API_KEY` | auto-detect | API key for model routing |
| `ORA_API_BASE` | `https://api.deepseek.com` | API base URL |
| `ORA_MODE` | `balanced` | Default mode: cheap, balanced, deep |

Auto-detects `DEEPSEEK_API_KEY`, `OPENAI_API_KEY`, `OPENROUTER_API_KEY` as fallbacks.
