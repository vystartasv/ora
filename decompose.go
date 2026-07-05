package ora

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os/exec"
	"strings"
	"time"
)

// DecomposeTask calls a local/API LLM to break a task into subtasks.
func DecomposeTask(cfg *Config, task, context string) []Subtask {
	prompt := DecompositionPrompt + "\n\nTASK: " + task
	if context != "" {
		prompt += "\n\nCONTEXT:\n" + context
	}

	// Try Ollama first (local, free, fast)
	result := callOllama(cfg, prompt)
	if result == "" {
		// Fallback to API
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
	model := strings.SplitN(cfg.LocalModel, ":", 2)[0]
	if model == "" {
		model = "qwen2.5-coder"
	}

	// Check ollama binary exists
	if _, err := exec.LookPath("ollama"); err != nil {
		return ""
	}

	// Check model exists
	check := exec.Command("ollama", "list")
	var checkOut bytes.Buffer
	check.Stdout = &checkOut
	if err := check.Run(); err != nil {
		return ""
	}
	if !strings.Contains(checkOut.String(), model) {
		return ""
	}

	// Use ollama run with pipe and timeout
	cmd := exec.Command("ollama", "run", model)
	cmd.Stdin = strings.NewReader(prompt)

	var out bytes.Buffer
	cmd.Stdout = &out
	cmd.Stderr = nil

	// Use a timer to enforce timeout
	done := make(chan error, 1)
	go func() {
		done <- cmd.Run()
	}()

	select {
	case <-done:
		return strings.TrimSpace(out.String())
	case <-time.After(20 * time.Second):
		cmd.Process.Kill()
		return ""
	}
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

	// Try API first (better quality for execution), then Ollama
	result := callAPI(cfg, prompt)
	if result == "" {
		result = callOllama(cfg, prompt)
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
