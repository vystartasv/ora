// Package ora provides task decomposition, routing, and agent orchestration.
package ora

import (
	"encoding/json"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

// ═══════════════════════════════════════════════════════════════════════
// Types
// ═══════════════════════════════════════════════════════════════════════

// TaskType classifies subtasks for routing.
type TaskType string

const (
	TaskLookup       TaskType = "lookup"
	TaskResearch     TaskType = "research"
	TaskCodeGen      TaskType = "code_gen"
	TaskReview       TaskType = "review"
	TaskDebug        TaskType = "debug"
	TaskArchitecture TaskType = "architecture"
	TaskPlan         TaskType = "plan"
)

// Subtask is a single unit of work from decomposition.
type Subtask struct {
	ID          string   `json:"id"`
	Type        TaskType `json:"type"`
	Goal        string   `json:"goal"`
	Files       []string `json:"files"`
	DependsOn   []string `json:"depends_on"`
	ExitCriterion string `json:"exit"`
	EstimatedMin int     `json:"estimated_minutes"`
}

// Route describes which model tier a subtask should use.
type Route struct {
	Tier       string `json:"tier"`
	Model      string `json:"model"`
	CostFactor int    `json:"cost_factor"`
	Reason     string `json:"reason"`
}

// Result is the outcome of executing one subtask.
type Result struct {
	ID     string `json:"id"`
	Goal   string `json:"goal"`
	Type   string `json:"type"`
	Model  string `json:"model"`
	Tier   string `json:"tier"`
	Status string `json:"status"`
	Output string `json:"output"`
	Exit   string `json:"exit"`
}

// Report is the full orchestration output.
type Report struct {
	Task      string        `json:"task"`
	WorkDir   string        `json:"workdir"`
	Subtasks  []Subtask     `json:"subtasks"`
	Results   []Result      `json:"results"`
	Duration  string        `json:"duration"`
	Savings   int           `json:"savings_pct"`
	Passed    int           `json:"passed"`
	Failed    int           `json:"failed"`
}

// Config holds ORA configuration.
type Config struct {
	APIKey     string `json:"api_key"`
	APIBase    string `json:"api_base"`
	APIModel   string `json:"api_model"`
	LocalModel string `json:"local_model"`
	Mode       string `json:"mode"` // fast, balanced, deep
	Agent      string `json:"agent"`
}

// ═══════════════════════════════════════════════════════════════════════
// Defaults & Constants
// ═══════════════════════════════════════════════════════════════════════

var TaskRoutes = map[TaskType]struct{ Tier, Model, Desc string }{
	TaskLookup:       {"cheap", "deepseek-v4-flash", "Read files, search, git status"},
	TaskResearch:     {"mid", "deepseek-v4-flash", "Web search, docs, summarise"},
	TaskCodeGen:      {"mid", "deepseek-v4-flash", "Write implementation code"},
	TaskReview:       {"mid", "deepseek-v4-flash", "Review diff, check quality"},
	TaskDebug:        {"flagship", "deepseek-v4-pro", "Root cause analysis"},
	TaskArchitecture: {"flagship", "deepseek-v4-pro", "System design, tradeoffs"},
	TaskPlan:         {"mid", "deepseek-v4-flash", "Task decomposition, strategy"},
}

var Emoji = map[string]string{
	"lookup": "🔍", "research": "📚", "code_gen": "⚡",
	"review": "👁", "debug": "🐛", "architecture": "🏗", "plan": "📐",
	"done": "✅", "fail": "❌", "warn": "⚠️", "info": "💡",
	"time": "⏱", "token": "🪙", "recycle": "♻️", "rocket": "🚀",
}

// CompressionPrompt is injected into every subtask prompt.
const CompressionPrompt = `
CRITICAL — Token efficiency rules:
1. CAVEMAN: omit filler, pronouns, pleasantries. Fragments only.
2. PONTAIL: does this need to exist? → stdlib? → one line? → minimum.
3. RTK: group similar items, deduplicate, truncate redundancy.
4. Never cut: validation, error handling, security, or accessibility.
`

const DecompositionPrompt = `You are ORA, a task decomposition engine. Break complex tasks into independent verifiable subtasks.

Respond with ONLY a JSON array. Each element:
{
  "id": "A",
  "type": "lookup|research|code_gen|review|debug|architecture|plan",
  "goal": "one-sentence description",
  "files": ["path/to/file.py"],
  "depends_on": [],
  "exit": "how to verify success"
}

Rules:
- Each subtask 2-15 min
- Dependencies accurate for safe parallel execution
- Exit criteria verifiable without subjective judgement
- Return ONLY the JSON array`

// ═══════════════════════════════════════════════════════════════════════
// Config loading
// ═══════════════════════════════════════════════════════════════════════

func LoadConfig(path string) (*Config, error) {
	cfg := &Config{
		APIBase:    "https://api.deepseek.com/v1",
		APIModel:   "deepseek-v4-flash",
		LocalModel: "deepseek-v4-flash",
		Mode:       "balanced",
		Agent:      "auto",
	}

	// Read API key from env
	cfg.APIKey = os.Getenv("ORA_API_KEY")
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("DEEPSEEK_API_KEY")
	}
	if cfg.APIKey == "" {
		cfg.APIKey = os.Getenv("OPENAI_API_KEY")
	}
	if base := os.Getenv("ORA_API_BASE"); base != "" {
		cfg.APIBase = base
	}
	if model := os.Getenv("ORA_API_MODEL"); model != "" {
		cfg.APIModel = model
	}
	if mode := os.Getenv("ORA_MODE"); mode != "" {
		cfg.Mode = mode
	}

	// Try loading config file
	configPath := path
	if configPath == "" {
		home, _ := os.UserHomeDir()
		configPath = filepath.Join(home, ".ora", "config.json")
	}
	if data, err := os.ReadFile(configPath); err == nil {
		json.Unmarshal(data, cfg)
	}

	return cfg, nil
}

// ═══════════════════════════════════════════════════════════════════════
// Utility
// ═══════════════════════════════════════════════════════════════════════

func DetectWorkdir() string {
	cwd, _ := os.Getwd()
	markers := []string{".git", "package.json", "Cargo.toml", "pyproject.toml", "go.mod"}
	for _, m := range markers {
		if _, err := os.Stat(filepath.Join(cwd, m)); err == nil {
			return cwd
		}
	}
	return cwd
}

func DetectAgent() string {
	for _, agent := range []string{"claude", "codex", "pi", "hermes"} {
		if _, err := exec.LookPath(agent); err == nil {
			return agent
		}
	}
	return ""
}

// GetRoute returns routing info for a task type.
func GetRoute(tt TaskType, mode string) Route {
	r, ok := TaskRoutes[tt]
	if !ok {
		r = TaskRoutes[TaskLookup]
	}

	tier := r.Tier
	switch mode {
	case "fast":
		tier = "cheap"
	case "deep":
		tier = "flagship"
	}

	costFactor := 1
	switch tier {
	case "mid":
		costFactor = 2
	case "flagship":
		costFactor = 10
	}

	model := r.Model
	if tier == "cheap" {
		model = "deepseek-v4-flash"
	} else if tier == "flagship" {
		model = "deepseek-v4-pro"
	}

	return Route{Tier: tier, Model: model, CostFactor: costFactor, Reason: r.Desc}
}

// DetectLocalModels returns available local LLM models from Ollama and oMLX.
func DetectLocalModels() []string {
	var models []string

	// 1. Check oMLX API at default local address
	resp, err := http.Get("http://127.0.0.1:8000/v1/models")
	if err == nil {
		defer resp.Body.Close()
		var data struct {
			Data []struct {
				ID string `json:"id"`
			} `json:"data"`
		}
		if json.NewDecoder(resp.Body).Decode(&data) == nil {
			for _, m := range data.Data {
				models = append(models, "omlx:"+m.ID)
			}
		}
	}

	// 2. Check Ollama
	if exec.Command("ollama", "list").Run() == nil {
		out, err := exec.Command("ollama", "list").Output()
		if err == nil {
			for _, line := range strings.Split(string(out), "\n") {
				parts := strings.Fields(line)
				if len(parts) >= 1 && parts[0] != "NAME" {
					models = append(models, "ollama:"+parts[0])
				}
			}
		}
	}

	return models
}

// PickSmallestModel returns the smallest available model (best for summaries).
func PickSmallestModel(models []string) string {
	prefs := []string{"1.5b", "tiny", "phi", "gemma2:2b", "qwen2.5:0.5b",
		"smollm", "llama3.2", "3b", "4b", "7b"}
	for _, m := range models {
		low := strings.ToLower(m)
		for _, p := range prefs {
			if strings.Contains(low, p) {
				return m
			}
		}
	}
	if len(models) > 0 {
		return models[0]
	}
	return ""
}

// TopologicalSort orders subtasks by dependency.
func TopologicalSort(tasks []Subtask) []Subtask {
	var sorted []Subtask
	completed := make(map[string]bool)
	remaining := make([]Subtask, len(tasks))
	copy(remaining, tasks)

	for len(remaining) > 0 {
		var batch []Subtask
		for _, t := range remaining {
			ready := true
			for _, d := range t.DependsOn {
				if !completed[d] {
					ready = false
					break
				}
			}
			if ready {
				batch = append(batch, t)
			}
		}
		if len(batch) == 0 {
			// Circular deps — append remaining
			sorted = append(sorted, remaining...)
			break
		}
		for _, t := range batch {
			sorted = append(sorted, t)
			completed[t.ID] = true
			for i, r := range remaining {
				if r.ID == t.ID {
					remaining = append(remaining[:i], remaining[i+1:]...)
					break
				}
			}
		}
	}
	return sorted
}

// SaveReport saves the orchestration report as JSON.
func SaveReport(report *Report, path string) error {
	data, err := json.MarshalIndent(report, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(path, data, 0644)
}

// GuessTaskType infers a TaskType from task description.
func GuessTaskType(task string) TaskType {
	low := strings.ToLower(task)
	switch {
	case containsAny(low, "research", "search", "find", "lookup"):
		return TaskResearch
	case containsAny(low, "debug", "fix", "bug", "error", "broken"):
		return TaskDebug
	case containsAny(low, "design", "architecture", "plan"):
		return TaskArchitecture
	case containsAny(low, "review", "check", "audit"):
		return TaskReview
	default:
		return TaskCodeGen
	}
}

func containsAny(s string, substrs ...string) bool {
	for _, sub := range substrs {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

// Color wrappers for terminal output.
var useColor = true

func init() {
	info, err := os.Stdout.Stat()
	if err != nil || (info.Mode()&os.ModeCharDevice) == 0 {
		useColor = false
	}
}

func colorize(s, code string) string {
	if !useColor {
		return s
	}
	return "\033[" + code + "m" + s + "\033[0m"
}
func ColorGreen(s string) string   { return colorize(s, "32") }
func ColorRed(s string) string     { return colorize(s, "31") }
func ColorYellow(s string) string  { return colorize(s, "33") }
func ColorCyan(s string) string    { return colorize(s, "36") }
func ColorBold(s string) string    { return colorize(s, "1") }

func isTerminal() bool {
	info, err := os.Stdout.Stat()
	if err != nil {
		return false
	}
	return (info.Mode() & os.ModeCharDevice) != 0
}
