package main

import (
	"flag"
	"fmt"
	"os"
	"os/exec"
	"strings"

	"github.com/vystartasv/ora"
)

func main() {
	planFlag := flag.Bool("plan", false, "Decompose only, no execution")
	fastFlag := flag.Bool("fast", false, "Force cheapest models")
	deepFlag := flag.Bool("deep", false, "Force flagship models")
	agentFlag := flag.String("agent", "auto", "Agent: hermes, claude, pi, codex")
	mcpFlag := flag.Bool("mcp", false, "MCP stdio mode (for Claude Code, Cursor)")
	serveFlag := flag.Bool("serve", false, "Start MCP HTTP server")
	portFlag := flag.Int("port", 8932, "MCP server port (HTTP mode)")
	versionFlag := flag.Bool("version", false, "Show version")
	contextFlag := flag.String("context", "", "Project context")

	flag.Usage = func() {
		fmt.Println(`ORA — Universal Task Orchestrator`)
		fmt.Println()
		fmt.Println(`Usage:`)
		fmt.Println(`  ora "build auth"                   Full pipeline`)
		fmt.Println(`  ora --plan "refactor API"           Plan only`)
		fmt.Println(`  ora --mcp                           MCP stdio mode (Claude Code)`)
		fmt.Println(`  ora --serve                         MCP HTTP server`)
		fmt.Println(`  ora init                            Setup config + ORA.md`)
		fmt.Println(`  ora init hermes                     Wire into Hermes Agent`)
		fmt.Println(`  ora init claude                     Wire into Claude Code`)
		fmt.Println(`  ora init pi                         Wire into Pi Agent`)
		fmt.Println(`  ora init all                        Wire into all detected agents`)
		fmt.Println()
		flag.PrintDefaults()
	}

	flag.Parse()
	args := flag.Args()

	// init / install commands
	if len(args) > 0 && (args[0] == "init" || args[0] == "install") {
		target := "default"
		if len(args) > 1 {
			target = args[1]
		}
		cmdInit(target)
		return
	}

	// MCP server — stdio mode (for Claude Code, Cursor, etc.)
	if *mcpFlag {
		cfg, _ := ora.LoadConfig("")
		ora.StartMCPStdio(cfg)
		return
	}

	// MCP server — HTTP mode
	if *serveFlag {
		cfg, _ := ora.LoadConfig("")
		if err := ora.StartMCPServer(cfg, *portFlag); err != nil {
			fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		return
	}

	if *versionFlag {
		fmt.Println("ORA v0.1.0")
		return
	}

	// Read task from args or pipe
	task := strings.Join(args, " ")
	if task == "" {
		stat, _ := os.Stdin.Stat()
		if (stat.Mode() & os.ModeCharDevice) == 0 {
			data := make([]byte, 4096)
			n, _ := os.Stdin.Read(data)
			task = strings.TrimSpace(string(data[:n]))
		}
	}
	if task == "" {
		flag.Usage()
		os.Exit(0)
	}

	cfg, err := ora.LoadConfig("")
	if err != nil {
		fmt.Fprintf(os.Stderr, "Warning: %v\n", err)
	}

	if *fastFlag {
		cfg.Mode = "fast"
	} else if *deepFlag {
		cfg.Mode = "deep"
	}
	if *agentFlag != "auto" {
		cfg.Agent = *agentFlag
	}

	ora.PrintBanner()
	fmt.Printf("  💡 Task: %s\n", task)
	fmt.Printf("  🚀 Dir:  %s\n", ora.DetectWorkdir())
	if det := ora.DetectAgent(); det != "" {
		fmt.Printf("  💡 Mode: %s | Agent: %s\n", cfg.Mode, det)
	} else {
		fmt.Printf("  💡 Mode: %s\n", cfg.Mode)
	}

	os.Exit(ora.Orchestrate(cfg, task, *contextFlag, *planFlag))
}

func cmdInit(target string) {
	switch target {
	case "hermes":
		installHermes()
	case "claude":
		installClaude()
	case "pi":
		installPi()
	case "all":
		installAll()
	default:
		installDefault()
	}
}

func findBinary() string {
	path, err := os.Executable()
	if err == nil {
		return path
	}
	if p, err := exec.LookPath("ora"); err == nil {
		return p
	}
	return ""
}

func installDefault() {
	home, _ := os.UserHomeDir()
	oraDir := home + "/.ora"
	os.MkdirAll(oraDir, 0755)

	// Config
	cfgPath := oraDir + "/config.json"
	if _, err := os.Stat(cfgPath); os.IsNotExist(err) {
		os.WriteFile(cfgPath, []byte(`{
  "mode": "balanced",
  "agent": "auto",
  "api_base": "https://api.deepseek.com/v1",
  "api_model": "deepseek-v4-flash",
  "local_model": "deepseek-v4-flash"
}`), 0644)
		fmt.Println("  ✅ Config: " + cfgPath)
	} else {
		fmt.Println("  💡 Config exists: " + cfgPath)
	}

	// ORA.md
	oraFile := "ORA.md"
	if _, err := os.Stat(oraFile); os.IsNotExist(err) {
		os.WriteFile(oraFile, []byte(`# ORA — Universal Task Orchestrator

Drop this file into ANY agent. Teaches decomposition, routing, compression.

## Core principle
Complex task → decompose → route each subtask to cheapest adequate model
→ delegate → compress → recompose → verify.

## Workflow
1. Triage — simple? inline. Complex? decompose.
2. Decompose — independent verifiable subtasks (2-15 min)
3. Route — lookup/research: cheap | code_gen/review: mid | debug/architecture: flagship
4. Compress — caveman fragments, ponytail YAGNI ladder, RTK grouping
5. Recombine — verify exit criteria, run tests, report
`), 0644)
		fmt.Println("  ✅ ORA.md written to current directory")
	}

	fmt.Println("\n  " + ora.ColorBold("ORA is ready."))
	fmt.Println("  Run: ora \"your task\"")
	fmt.Println("  Run: ora init hermes | claude | pi | all")
}

func installHermes() {
	home, _ := os.UserHomeDir()
	hermesDir := home + "/.hermes"

	// 1. Create plugin
	pluginDir := hermesDir + "/plugins/ora"
	os.MkdirAll(pluginDir, 0755)

	pluginYaml := `name: ora
description: "Universal Task Orchestrator — decompose, route, delegate"
version: 0.1.0
hooks:
  pre_llm_call:
    - id: ora_inject
      priority: 50
commands:
  ora:
    description: Decompose a task with ORA
  ora-route:
    description: Check routing for a task type
instructions: |
  ORA is active. On complex tasks: decompose → route → delegate → compress → verify.
  Route: lookup/research → cheap, code_gen/review → mid, debug/architecture → flagship.
  Compression mandatory: caveman fragments, ponytail ladder, RTK grouping.
`
	os.WriteFile(pluginDir+"/plugin.yaml", []byte(pluginYaml), 0644)
	fmt.Println("  ✅ Plugin: " + pluginDir)

	// 2. Register MCP server — write directly to config.yaml
	if binary := findBinary(); binary != "" {
		cfgPath := hermesDir + "/config.yaml"
		if data, err := os.ReadFile(cfgPath); err == nil {
			content := string(data)
			if !strings.Contains(content, "command:") || !strings.Contains(content, "ora") {
				mcpBlock := fmt.Sprintf(`
  ora:
    command: %s
    args: ["--mcp"]
    connect_timeout: 10
`, binary)
				if idx := strings.Index(content, "mcp_servers:"); idx >= 0 {
					insertPoint := idx + 12
					if endIdx := strings.Index(content[insertPoint:], "\n  "); endIdx >= 0 && endIdx < 20 {
						content = content[:insertPoint+endIdx] + mcpBlock + content[insertPoint+endIdx:]
					} else {
						content += mcpBlock
					}
				} else {
					content += "\nmcp_servers:" + mcpBlock
				}
				os.WriteFile(cfgPath, []byte(content), 0644)
				fmt.Println("  ✅ MCP: ORA server added to " + cfgPath)
			} else {
				fmt.Println("  💡 MCP already configured in " + cfgPath)
			}
		} else {
			fmt.Println("  ⚠️  Could not read " + cfgPath)
		}
	}

	// 3. Copy ORA.md to Hermes dir
	oraMd := hermesDir + "/ORA.md"
	os.WriteFile(oraMd, []byte(`# ORA — Always active in Hermes

ORA provides MCP tools: ora_decompose, ora_route, ora_execute.

## Decomposition workflow
On complex tasks: call ora_decompose → review plan → route each subtask → execute cheapest-first → verify.

## Routing
- lookup/research → cheap (deepseek-v4-flash or local)
- code_gen/review → mid (deepseek-v4-flash)
- debug/architecture → flagship (deepseek-v4-pro)

## Compression
1. Omit filler. Fragments.
2. YAGNI ladder: exist? → codebase? → stdlib? → native? → one line? → minimum.
3. Never cut: validation, error handling, security, accessibility.
`), 0644)
	fmt.Println("  ✅ Instructions: " + oraMd)

	fmt.Println("\n  " + ora.ColorBold("ORA wired into Hermes."))
	fmt.Println("  Restart Hermes: hermes")
	fmt.Println("  Commands: /ora, /ora-route")
}

func installClaude() {
	home, _ := os.UserHomeDir()

	// 1. Register MCP server in stdio mode
	claudeDir := home + "/.claude"
	os.MkdirAll(claudeDir, 0755)

	if binary := findBinary(); binary != "" {
		// Write Claude Code MCP config directly
		mcpConfig := fmt.Sprintf(`{
  "mcpServers": {
    "ora": {
      "command": "%s",
      "args": ["--mcp"]
    }
  }
}`, binary)

		settingsPath := claudeDir + "/mcp.json"
		os.WriteFile(settingsPath, []byte(mcpConfig), 0644)
		fmt.Println("  ✅ MCP config: " + settingsPath)

		// Also try claude CLI
		cmd2 := exec.Command("claude", "mcp", "add", "ora",
			"--", binary, "--mcp")
		if _, err := cmd2.CombinedOutput(); err == nil {
			fmt.Println("  ✅ MCP: ora registered with Claude Code CLI")
		} else {
			fmt.Printf("  💡 Claude MCP config written to %s\n", settingsPath)
		}
	}

	// 2. Copy ORA.md as CLAUDE.md in current dir
	oraFile := "CLAUDE.md"
	if _, err := os.Stat(oraFile); os.IsNotExist(err) {
		os.WriteFile(oraFile, []byte(`# ORA Instructions for Claude Code

ORA is active via MCP. You have these MCP tools:
  - ora_decompose: break a task into subtasks
  - ora_route: check what model a task type should use
  - ora_execute: run a subtask on the cheapest adequate model

## Decomposition workflow
On complex tasks (3+ files, architecture-level, multi-step):
1. Call ora_decompose with the task description
2. Review the subtask plan
3. For each subtask, call ora_route to check what model tier to use
4. Execute subtasks in dependency order, cheapest model first
5. Verify results, don't trust self-reports

## Routing (use as guidance)
- lookup/research → cheap (haiku / deepseek-v4-flash)
- code_gen/review → mid (sonnet / deepseek-v4-flash)
- debug/architecture → flagship (sonnet / deepseek-v4-pro)

## Compression (always apply)
1. Omit filler. Fragments.
2. YAGNI ladder: exist? → stdlib? → one line? → minimum.
3. Never cut: validation, error handling, security, accessibility.
`), 0644)
		fmt.Println("  ✅ " + oraFile + " written")
	}

	fmt.Println("\n  " + ora.ColorBold("ORA wired into Claude Code."))
	fmt.Println("  Run: ora init pi  to also wire into Pi")
}

func installPi() {
	home, _ := os.UserHomeDir()

	// 1. Register MCP server in Pi settings
	piDir := home + "/.pi/agent"
	os.MkdirAll(piDir, 0755)

	if binary := findBinary(); binary != "" {
		// Register MCP server with Pi via settings
		mcpConfig := fmt.Sprintf(`{
  "mcpServers": {
    "ora": {
      "command": "%s",
      "args": ["--mcp"]
    }
  }
}`, binary)

		settingsPath := piDir + "/mcp.json"
		os.WriteFile(settingsPath, []byte(mcpConfig), 0644)
		fmt.Println("  ✅ MCP: " + settingsPath)

		// Also try pi command
		cmd := exec.Command("pi", "mcp", "add", "ora", "--", binary, "--serve")
		if out, err := cmd.CombinedOutput(); err == nil {
			fmt.Println("  ✅ MCP: ora registered with Pi via CLI")
		} else {
			_ = out  // used in scoping, discard for fallback message
			fmt.Printf("  💡 Pi MCP config written to %s\n", settingsPath)
		}
	}

	fmt.Println("\n  " + ora.ColorBold("ORA wired into Pi Agent."))
	fmt.Println("  Restart Pi for MCP to take effect.")
}

func installAll() {
	fmt.Println("  " + ora.ColorBold("Installing ORA for all detected agents..."))
	fmt.Println()

	installed := 0

	// Hermes
	if _, err := exec.LookPath("hermes"); err == nil {
		installHermes()
		installed++
	} else {
		fmt.Println("  ⚠️  Hermes not found — skipping")
	}

	// Claude Code
	if _, err := exec.LookPath("claude"); err == nil {
		installClaude()
		installed++
	} else {
		fmt.Println("  ⚠️  Claude Code not found — skipping")
	}

	// Pi
	if _, err := exec.LookPath("pi"); err == nil {
		installPi()
		installed++
	} else {
		fmt.Println("  ⚠️  Pi not found — skipping")
	}

	if installed == 0 {
		fmt.Println()
		installDefault()
	} else {
		fmt.Printf("\n  %s ORA wired into %d agent(s). Ready.\n", ora.ColorBold("✅"), installed)
	}
}
