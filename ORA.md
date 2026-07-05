# ORA.md — Universal Task Orchestrator Prompt

Drop this file into ANY agent that reads rules files.
Teaches decomposition, model routing, compression, and recomposition.

## Core principle
When given a complex task: decompose → route each subtask to cheapest adequate model → delegate → compress → recompose → verify.

## The workflow

1. **Triage** — simple? handle inline. Complex? run the full pipeline.
2. **Decompose** — break into independent verifiable subtasks (2-15 min each)
3. **Route** — map each subtask to cheapest adequate model:
   - lookup/research: cheap tier (Flash, 3B local)
   - code_gen/review: mid tier (Flash, 7B)
   - debug/architecture: flagship tier (Pro, Sonnet)
4. **Delegate** — spawn subagents via CLI or native subagent support
5. **Compress** — caveman fragments, ponytail YAGNI ladder, RTK grouping
6. **Recompose** — verify exit criteria, run tests, report

## Routing matrix

| Task type | Default tier | Cost vs all-flagship |
|-----------|-------------|---------------------|
| lookup, research | cheap | ~10x cheaper |
| code_gen, review | mid | ~5x cheaper |
| debug, architecture | flagship | correct tool |

## Token efficiency (mandatory)

1. Omit filler, pronouns, pleasantries. Fragments only.
2. Does this need to exist? → already in codebase? → stdlib? → native? → dependency? → one line? → minimum.
3. Group similar items, deduplicate, truncate redundancy.
4. Never cut: validation, error handling, security, accessibility.

## Exit criteria

Every subtask must have a verifiable exit criterion:
- File exists: path/to/file.ext
- Command passes: `go test ./...`
- Pattern found: `grep -q "func" file.go`

Do not trust self-reports. Verify.
