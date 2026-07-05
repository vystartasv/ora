package ora

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"sort"
	"strings"
	"time"
)

// DecomposeTask calls a local/API LLM to break a task into subtasks.
func DecomposeTask(cfg *Config, task, context string) []Subtask {
	prompt := DecompositionPrompt + "\n\nTASK: " + task
	if context != "" {
		prompt += "\n\nCONTEXT:\n" + context
	}

	// Try oMLX first (fast local), then Ollama, then API
	result := callOMLX(prompt)
	if result == "" {
		result = callOllama(cfg, prompt)
	}
	if result == "" {
		result = callAPI(cfg, prompt)
	}

	if result == "" {
		return nil
	}

	return parseSubtasks(result)
}

func parseSubtasks(raw string) []Subtask {
	// Try to extract JSON array
	start := strings.Index(raw, "[")
	end := strings.LastIndex(raw, "]")
	if start >= 0 && end > start {
		raw = raw[start : end+1]
	}

	var tasks []Subtask
	if err := json.Unmarshal([]byte(raw), &tasks); err == nil && len(tasks) > 0 {
		return tasks
	}

	// Try object with subtasks key
	var obj map[string]json.RawMessage
	if err := json.Unmarshal([]byte(raw), &obj); err == nil {
		if data, ok := obj["subtasks"]; ok {
			if err := json.Unmarshal(data, &tasks); err == nil {
				return tasks
			}
		}
	}

	return nil
}

func callOllama(cfg *Config, prompt string) string {
	// Check ollama binary exists
	if _, err := exec.LookPath("ollama"); err != nil {
		return ""
	}

	// Get available models
	models := getOllamaModels()
	if len(models) == 0 {
		return ""
	}

	// Try each model, smallest first (prefer Qwen, Gemma, Phi for speed)
	for _, model := range models {
		// Use ollama run with pipe and timeout
		cmd := exec.Command("ollama", "run", model)
		cmd.Stdin = strings.NewReader(prompt)

		var out bytes.Buffer
		cmd.Stdout = &out
		cmd.Stderr = nil

		done := make(chan error, 1)
		go func() {
			done <- cmd.Run()
		}()

		select {
		case <-done:
			if s := strings.TrimSpace(out.String()); s != "" {
				return s
			}
		case <-time.After(20 * time.Second):
			cmd.Process.Kill()
		}
	}
	return ""
}

func getOllamaModels() []string {
	out, err := exec.Command("ollama", "list").Output()
	if err != nil {
		return nil
	}
	var models []string
	for _, line := range strings.Split(string(out), "\n") {
		parts := strings.Fields(line)
		if len(parts) >= 1 && parts[0] != "NAME" {
			models = append(models, parts[0])
		}
	}
	// Sort small models first
	sortModelsBySize(models)
	return models
}

func sortModelsBySize(models []string) {
	sort.Slice(models, func(i, j int) bool {
		pref := []string{"0.5b", "1.5b", "2b", "3b", "4b", "7b", "8b", "9b", "13b", "14b", "20b", "30b", "70b"}
		gi, gj := 99, 99
		for k, p := range pref {
			if strings.Contains(strings.ToLower(models[i]), p) {
				gi = k
			}
			if strings.Contains(strings.ToLower(models[j]), p) {
				gj = k
			}
		}
		return gi < gj
	})
}

func callOMLX(prompt string) string {
	payload := map[string]interface{}{
		"model": "", // let server decide default
		"messages": []map[string]string{
			{"role": "user", "content": prompt},
		},
		"max_tokens": 4000,
		"temperature": 0.2,
	}
	data, _ := json.Marshal(payload)
	resp, err := http.Post("http://127.0.0.1:8000/v1/chat/completions",
		"application/json", bytes.NewReader(data))
	if err != nil {
		return ""
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if json.Unmarshal(body, &result) == nil && len(result.Choices) > 0 {
		return strings.TrimSpace(result.Choices[0].Message.Content)
	}
	return ""
}

func callAPI(cfg *Config, prompt string) string {
	if cfg.APIKey == "" {
		return ""
	}

	payload := map[string]interface{}{
		"model": cfg.APIModel,
		"messages": []map[string]string{
			{"role": "system", "content": strings.TrimSpace(CompressionPrompt)},
			{"role": "user", "content": prompt},
		},
		"max_tokens": 4000,
		"temperature": 0.2,
	}

	data, _ := json.Marshal(payload)
	req, err := http.NewRequest("POST", cfg.APIBase+"/chat/completions", bytes.NewReader(data))
	if err != nil {
		return ""
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+cfg.APIKey)

	client := &http.Client{Timeout: 45 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return ""
	}
	defer resp.Body.Close()

	body, _ := io.ReadAll(resp.Body)
	var result struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.Unmarshal(body, &result); err != nil || len(result.Choices) == 0 {
		return ""
	}

	return strings.TrimSpace(result.Choices[0].Message.Content)
}

// ExecuteSubtask calls an LLM to perform a single subtask.
func ExecuteSubtask(cfg *Config, subtask Subtask, workdir string) Result {
	route := GetRoute(subtask.Type, cfg.Mode)

	prompt := fmt.Sprintf(`Task: %s

Working directory: %s

Files involved:
%s

Exit criterion: %s

Instructions:
1. Read relevant files before making changes
2. Write minimal code — no over-engineering
3. Verify against exit criterion
4. Report: what was done, any issues, is exit criterion met
%s`,
		subtask.Goal, workdir,
		"  "+strings.Join(subtask.Files, "\n  "),
		subtask.ExitCriterion,
		CompressionPrompt,
	)

	// Try oMLX → Ollama → API (cheapest first)
	result := callOMLX(prompt)
	if result == "" {
		result = callOllama(cfg, prompt)
	}
	if result == "" {
		result = callAPI(cfg, prompt)
	}

	status := "completed"
	if result == "" {
		status = "failed"
	} else if strings.Contains(strings.ToLower(result), "exit criterion is met") ||
		strings.Contains(strings.ToLower(result), "completed") {
		status = "completed"
	} else if strings.Contains(strings.ToLower(result), "fail") {
		status = "failed"
	} else {
		status = "uncertain"
	}

	if len(result) > 1500 {
		result = result[:1500] + "..."
	}

	return Result{
		ID:     subtask.ID,
		Goal:   subtask.Goal,
		Type:   string(subtask.Type),
		Model:  route.Model,
		Tier:   route.Tier,
		Status: status,
		Output: result,
		Exit:   subtask.ExitCriterion,
	}
}

// ExecuteViaAgent spawns a CLI coding agent to perform a subtask.
func ExecuteViaAgent(cfg *Config, subtask Subtask, workdir, agent string) Result {
	route := GetRoute(subtask.Type, cfg.Mode)

	// Build prompt with compression
	prompt := subtask.Goal + "\n\n" + strings.TrimSpace(CompressionPrompt)
	if subtask.ExitCriterion != "" {
		prompt += "\nExit: " + subtask.ExitCriterion
	}
	prompt += "\nDir: " + workdir

	// Escape for shell
	safePrompt := strings.ReplaceAll(prompt, "'", "'\\''")

	var cmdStr string
	switch agent {
	case "pi":
		cmdStr = fmt.Sprintf("pi -p --model Qwen3.5-4B-OptiQ-4bit '%s'", safePrompt)
	case "claude":
		model := "haiku"
		if subtask.Type == TaskDebug || subtask.Type == TaskArchitecture {
			model = "sonnet"
		}
		cmdStr = fmt.Sprintf("claude -p '%s' --model %s --allowedTools Read,Edit,Write", safePrompt, model)
	case "codex":
		cmdStr = fmt.Sprintf("codex exec '%s' --yolo", safePrompt)
	case "hermes":
		model := "deepseek-v4-flash"
		if subtask.Type == TaskDebug || subtask.Type == TaskArchitecture {
			model = "deepseek-v4-pro"
		}
		cmdStr = fmt.Sprintf("hermes chat -q '%s' --provider deepseek --model %s -Q", safePrompt, model)
	default:
		cmdStr = fmt.Sprintf("pi -p --model Qwen3.5-4B-OptiQ-4bit '%s'", safePrompt)
	}

	// Execute
	cmd := exec.Command("sh", "-c", cmdStr)
	cmd.Dir = workdir
	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = &out

	err := cmd.Run()
	output := strings.TrimSpace(out.String())
	if len(output) > 1500 {
		output = output[:1500] + "..."
	}

	status := "completed"
	if err != nil {
		status = "uncertain"
	}
	if output == "" {
		status = "failed"
	}

	modelName := fmt.Sprintf("%s/%s", agent, route.Model)
	return Result{
		ID:     subtask.ID,
		Goal:   subtask.Goal,
		Type:   string(subtask.Type),
		Model:  modelName,
		Tier:   route.Tier,
		Status: status,
		Output: output,
		Exit:   subtask.ExitCriterion,
	}
}
