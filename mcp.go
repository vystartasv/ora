package ora

import (
	"encoding/json"
	"fmt"
	"io"
	"net/http"
)

// ═══════════════════════════════════════════════════════════════════════
// MCP Server — provides ORA tools to any MCP-capable agent
// ═══════════════════════════════════════════════════════════════════════

type mcpRequest struct {
	JSONRPC string          `json:"jsonrpc"`
	ID      int             `json:"id"`
	Method  string          `json:"method"`
	Params  json.RawMessage `json:"params,omitempty"`
}

type mcpResponse struct {
	JSONRPC string      `json:"jsonrpc"`
	ID      int         `json:"id"`
	Result  interface{} `json:"result,omitempty"`
	Error   *mcpError   `json:"error,omitempty"`
}

type mcpError struct {
	Code    int    `json:"code"`
	Message string `json:"message"`
}

type mcpCallParams struct {
	Name      string          `json:"name"`
	Arguments json.RawMessage `json:"arguments"`
}

type mcpToolResult struct {
	Content []mcpContent `json:"content"`
}

type mcpContent struct {
	Type string `json:"type"`
	Text string `json:"text"`
}

// MCPHandler handles MCP JSON-RPC requests.
func MCPHandler(w http.ResponseWriter, r *http.Request, cfg *Config) {
	body, err := io.ReadAll(r.Body)
	if err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	var req mcpRequest
	if err := json.Unmarshal(body, &req); err != nil {
		http.Error(w, err.Error(), 400)
		return
	}

	resp := handleMCP(req, cfg)
	w.Header().Set("Content-Type", "application/json")
	json.NewEncoder(w).Encode(resp)
}

func handleMCP(req mcpRequest, cfg *Config) mcpResponse {
	switch req.Method {
	case "initialize":
		return mcpResponse{
			JSONRPC: "2.0", ID: req.ID,
			Result: map[string]interface{}{
				"protocolVersion": "2025-03-26",
				"capabilities": map[string]interface{}{
					"tools": map[string]interface{}{
						"ora_decompose": map[string]interface{}{
							"description": "Decompose a task into independent subtasks",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"task":    map[string]interface{}{"type": "string"},
									"context": map[string]interface{}{"type": "string"},
								},
								"required": []string{"task"},
							},
						},
						"ora_route": map[string]interface{}{
							"description": "Route a task type to the cheapest adequate model",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"task_type": map[string]interface{}{
										"type": "string",
										"enum": []string{"lookup", "research", "code_gen", "review", "debug", "architecture", "plan"},
									},
								},
								"required": []string{"task_type"},
							},
						},
						"ora_execute": map[string]interface{}{
							"description": "Execute a subtask via the cheapest adequate model",
							"inputSchema": map[string]interface{}{
								"type": "object",
								"properties": map[string]interface{}{
									"goal":     map[string]interface{}{"type": "string"},
									"type":     map[string]interface{}{"type": "string"},
									"files":    map[string]interface{}{"type": "array", "items": map[string]interface{}{"type": "string"}},
									"exit":     map[string]interface{}{"type": "string"},
									"workdir":  map[string]interface{}{"type": "string"},
								},
								"required": []string{"goal"},
							},
						},
					},
				},
			},
		}

	case "tools/call":
		var params mcpCallParams
		if err := json.Unmarshal(req.Params, &params); err != nil {
			return mcpErrorResponse(req.ID, -32602, "Invalid params")
		}

		switch params.Name {
		case "ora_decompose":
			var args struct {
				Task    string `json:"task"`
				Context string `json:"context"`
			}
			json.Unmarshal(params.Arguments, &args)
			tasks := DecomposeTask(cfg, args.Task, args.Context)
			data, _ := json.MarshalIndent(tasks, "", "  ")
			return mcpResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: mcpToolResult{Content: []mcpContent{{Type: "text", Text: string(data)}}},
			}

		case "ora_route":
			var args struct {
				TaskType string `json:"task_type"`
			}
			json.Unmarshal(params.Arguments, &args)
			route := GetRoute(TaskType(args.TaskType), cfg.Mode)
			data, _ := json.MarshalIndent(route, "", "  ")
			return mcpResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: mcpToolResult{Content: []mcpContent{{Type: "text", Text: string(data)}}},
			}

		case "ora_execute":
			var args struct {
				Goal    string   `json:"goal"`
				Type    string   `json:"type"`
				Files   []string `json:"files"`
				Exit    string   `json:"exit"`
				Workdir string   `json:"workdir"`
			}
			json.Unmarshal(params.Arguments, &args)
			if args.Workdir == "" {
				args.Workdir = DetectWorkdir()
			}
			if args.Type == "" {
				args.Type = "code_gen"
			}

			result := ExecuteSubtask(cfg, Subtask{
				ID: "0", Type: TaskType(args.Type), Goal: args.Goal,
				Files: args.Files, ExitCriterion: args.Exit,
			}, args.Workdir)

			data, _ := json.MarshalIndent(result, "", "  ")
			return mcpResponse{
				JSONRPC: "2.0", ID: req.ID,
				Result: mcpToolResult{Content: []mcpContent{{Type: "text", Text: string(data)}}},
			}
		}
	}

	return mcpErrorResponse(req.ID, -32601, "Method not found")
}

func mcpErrorResponse(id int, code int, msg string) mcpResponse {
	return mcpResponse{
		JSONRPC: "2.0", ID: id,
		Error: &mcpError{Code: code, Message: msg},
	}
}

// StartMCPServer starts the MCP HTTP server.
func StartMCPServer(cfg *Config, port int) error {
	handler := func(w http.ResponseWriter, r *http.Request) {
		MCPHandler(w, r, cfg)
	}

	http.HandleFunc("/mcp", handler)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, "ORA MCP Server — use POST /mcp with JSON-RPC 2.0\n")
	})

	addr := fmt.Sprintf(":%d", port)
	fmt.Printf("ORA MCP Server running on http://0.0.0.0%s/mcp\n", addr)
	return http.ListenAndServe(addr, nil)
}
