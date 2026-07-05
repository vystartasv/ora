package ora

import (
	"fmt"
	"strings"
	"time"
)

// Orchestrate runs the full pipeline: decompose → route → execute → reconcile.
func Orchestrate(cfg *Config, task, context string, planOnly bool) int {
	workdir := DetectWorkdir()
	agent := cfg.Agent
	if agent == "auto" {
		agent = DetectAgent()
	}

	start := time.Now()

	// Phase 1: Decompose
	fmt.Print("\n  " + Emoji["time"] + "  Analysing task...")

	subtasks := DecomposeTask(cfg, task, context)
	fmt.Print("\r") // Clear spinner

	if len(subtasks) == 0 {
		fmt.Printf("\n  %s  Simple task — executing inline\n\n", Emoji["warn"])
		result := ExecuteSubtask(cfg, Subtask{
			ID: "0", Type: GuessTaskType(task), Goal: task,
		}, workdir)
		PrintResults([]Result{result})
		elapsed := time.Since(start).Truncate(time.Second)
		fmt.Printf("\n  %s  %s\n", Emoji["time"], elapsed)
		if result.Status == "failed" {
			return 1
		}
		return 0
	}

	PrintHeader("DECOMPOSITION")
	PrintPlan(subtasks)

	if planOnly {
		fmt.Printf("\n  %s  Plan mode — re-run without --plan to execute\n\n", Emoji["info"])
		return 0
	}

	// Phase 2: Execute
	PrintHeader("EXECUTION")
	var results []Result
	completed := make(map[string]bool)
	ordered := TopologicalSort(subtasks)

	for _, subtask := range ordered {
		fmt.Printf("\n  %s  [%s] %s...\n", Emoji["time"], ColorBold(subtask.ID), subtask.Goal)

		var result Result
		if agent != "" {
			result = ExecuteViaAgent(cfg, subtask, workdir, agent)
		} else {
			result = ExecuteSubtask(cfg, subtask, workdir)
		}

		results = append(results, result)

		if result.Status == "completed" {
			completed[subtask.ID] = true
			fmt.Printf("  %s  [%s] Done (%s)\n", Emoji["done"], subtask.ID, result.Model)
		} else {
			fmt.Printf("  %s  [%s] %s\n", Emoji["warn"], subtask.ID, result.Status)
		}
	}

	// Phase 3: Reconcile
	elapsed := time.Since(start).Truncate(time.Second)
	passed := 0
	failed := 0
	for _, r := range results {
		switch r.Status {
		case "completed":
			passed++
		case "failed":
			failed++
		}
	}

	totalTasks := len(subtasks)
	allFlagshipCost := totalTasks * 10
	actualCost := 0
	for _, t := range subtasks {
		actualCost += GetRoute(t.Type, cfg.Mode).CostFactor
	}
	savings := 0
	if allFlagshipCost > 0 {
		savings = 100 - (actualCost * 100 / allFlagshipCost)
	}

	PrintHeader("RECONCILIATION")
	PrintResults(results)

	fmt.Printf("\n  %s  ~%d%% cheaper than all-flagship\n", Emoji["recycle"], savings)
	fmt.Printf("\n  %s  %s\n", Emoji["time"], elapsed)

	// Save report
	report := &Report{
		Task:     task,
		WorkDir:  workdir,
		Subtasks: subtasks,
		Results:  results,
		Duration: elapsed.String(),
		Savings:  savings,
		Passed:   passed,
		Failed:   failed,
	}
	reportPath := workdir + "/.ora-report.json"
	if err := SaveReport(report, reportPath); err == nil {
		fmt.Printf("\n  %s  Report saved to %s\n", Emoji["info"], reportPath)
	}

	if failed > 0 {
		return 1
	}
	return 0
}

// PrintHeader prints a section header.
func PrintHeader(title string) {
	w := 60
	fmt.Printf("\n  %s\n", strings.Repeat("─", w))
	fmt.Printf("  %s%s%s\n", ColorBold("  "), strings.ToUpper(title), strings.Repeat(" ", 3))
	fmt.Printf("  %s\n", strings.Repeat("─", w))
}

// PrintPlan displays the decomposition plan with routing.
func PrintPlan(subtasks []Subtask) {
	for i, t := range subtasks {
		route := GetRoute(t.Type, "balanced")
		emoji := Emoji[string(t.Type)]
		if emoji == "" {
			emoji = "•"
		}
		deps := ""
		if len(t.DependsOn) > 0 {
			deps = " → after: " + strings.Join(t.DependsOn, ",")
		}
		fmt.Printf("\n  %d. %s [%s] %s", i+1, emoji, ColorCyan(string(t.Type)), t.Goal)
		fmt.Printf("\n     %s Model: %s (tier: %s)%s", Emoji["token"], route.Model, route.Tier, deps)
		if t.ExitCriterion != "" {
			fmt.Printf("\n     %s %s", Emoji["done"], t.ExitCriterion)
		}
	}
	fmt.Println()
}

// PrintResults displays execution results.
func PrintResults(results []Result) {
	passed := 0
	failed := 0
	for _, r := range results {
		switch r.Status {
		case "completed":
			passed++
		case "failed":
			failed++
		}
	}

	for _, r := range results {
		icon := Emoji["done"]
		if r.Status == "failed" {
			icon = Emoji["fail"]
		} else if r.Status != "completed" {
			icon = Emoji["warn"]
		}
		fmt.Printf("\n  %s [%s] %s", icon, r.ID, r.Goal)
		fmt.Printf("\n     %s %s (%s) — %s", Emoji["token"], r.Model, r.Tier, r.Status)
	}

	fmt.Printf("\n\n  %s  %d/%d passed", Emoji["done"], passed, len(results))
	if failed > 0 {
		fmt.Printf(", %s %d failed", Emoji["fail"], failed)
	}
	fmt.Println()
}

// PrintBanner shows the ORA header.
func PrintBanner() {
	fmt.Printf("\n")
	fmt.Printf("  %s\n", strings.Repeat("═", 48))
	fmt.Printf("  %s\n", ColorBold("   ORA  —  Universal Task Orchestrator"))
	fmt.Printf("  %s\n", ColorCyan("   decompose · route · delegate · reconcile"))
	fmt.Printf("  %s\n", strings.Repeat("═", 48))
	fmt.Printf("\n")
}
