package agents

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"os"
	"strings"
)

const (
	openRouterEndpoint = "https://openrouter.ai/api/v1/chat/completions"
	chatModel          = "minimax/minimax-m2.7"
	maxAgentIterations = 12
)

// ChatMessage is a single turn in a chat conversation.
type ChatMessage struct {
	Role       string      `json:"role"`
	Content    interface{} `json:"content"` // string or nil when tool_calls present
	ToolCalls  []ToolCall  `json:"tool_calls,omitempty"`
	ToolCallID string      `json:"tool_call_id,omitempty"`
	Name       string      `json:"name,omitempty"`
}

// ChatClient drives the agentic loop against OpenRouter.
type ChatClient struct {
	apiKey string
	http   *http.Client
}

// NewChatClient reads OPENROUTER_API_KEY from the environment.
func NewChatClient() *ChatClient {
	return &ChatClient{
		apiKey: os.Getenv("OPENROUTER_API_KEY"),
		http:   &http.Client{},
	}
}

// Available reports whether an API key is configured.
func (c *ChatClient) Available() bool {
	return c.apiKey != ""
}

type completionResponse struct {
	Choices []struct {
		Message struct {
			Role      string     `json:"role"`
			Content   *string    `json:"content"`
			ToolCalls []ToolCall `json:"tool_calls"`
		} `json:"message"`
		FinishReason string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *ChatClient) complete(ctx context.Context, messages []ChatMessage, useTools bool) (*completionResponse, error) {
	body := map[string]interface{}{
		"model":    chatModel,
		"messages": messages,
	}
	if useTools {
		body["tools"] = toolDefs
		body["tool_choice"] = "auto"
	}

	payload, _ := json.Marshal(body)
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(payload))
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("HTTP-Referer", "https://github.com/zaycruz/linear-tui")
	req.Header.Set("X-Title", "linear-tui")

	resp, err := c.http.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()

	raw, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("API %s: %s", resp.Status, raw)
	}

	var cr completionResponse
	if err := json.Unmarshal(raw, &cr); err != nil {
		return nil, fmt.Errorf("parse response: %w", err)
	}
	if cr.Error != nil {
		return nil, fmt.Errorf("API error: %s", cr.Error.Message)
	}
	return &cr, nil
}

// RunAgent drives the full agentic loop.
// onStatus is called for tool call status messages shown in the chat pane.
// onDone is called with the final text response (or error).
// The call is non-blocking — runs in a goroutine.
func (c *ChatClient) RunAgent(
	ctx context.Context,
	system string,
	history []ChatMessage,
	executor ToolExecutor,
	onStatus func(string),
	onDone func(string, error),
) {
	go func() {
		msgs := make([]ChatMessage, 0, len(history)+1)
		if system != "" {
			msgs = append(msgs, ChatMessage{Role: "system", Content: system})
		}
		msgs = append(msgs, history...)

		for i := 0; i < maxAgentIterations; i++ {
			cr, err := c.complete(ctx, msgs, true)
			if err != nil {
				onDone("", err)
				return
			}
			if len(cr.Choices) == 0 {
				onDone("", fmt.Errorf("empty response from API"))
				return
			}

			choice := cr.Choices[0]

			// Final text response
			if choice.FinishReason == "stop" || len(choice.Message.ToolCalls) == 0 {
				content := ""
				if choice.Message.Content != nil {
					content = *choice.Message.Content
				}
				onDone(content, nil)
				return
			}

			// Append assistant message with tool calls
			msgs = append(msgs, ChatMessage{
				Role:      "assistant",
				Content:   nil,
				ToolCalls: choice.Message.ToolCalls,
			})

			// Execute each tool call
			for _, tc := range choice.Message.ToolCalls {
				args := tc.Args()
				onStatus(fmt.Sprintf("→ %s(%s)", tc.Function.Name, summarizeArgs(args)))

				result, execErr := executeToolCall(ctx, tc, executor)
				if execErr != nil {
					result = fmt.Sprintf("error: %s", execErr.Error())
				}

				msgs = append(msgs, ChatMessage{
					Role:       "tool",
					Content:    result,
					ToolCallID: tc.ID,
					Name:       tc.Function.Name,
				})
			}
		}

		onDone("", fmt.Errorf("agent exceeded max iterations (%d)", maxAgentIterations))
	}()
}

func executeToolCall(ctx context.Context, tc ToolCall, ex ToolExecutor) (string, error) {
	args := tc.Args()
	str := func(k string) string {
		v, _ := args[k].(string)
		return v
	}

	switch tc.Function.Name {
	case "update_issue":
		msg, err := ex.ExecUpdateIssue(str("issue_id"), args)
		return msg, err

	case "create_issue":
		pri := 0
		if v, ok := args["priority"].(float64); ok {
			pri = int(v)
		}
		id, err := ex.ExecCreateIssue(
			ex.ReadCurrentTeamID(),
			str("title"), str("description"), str("project"),
			str("status"), str("assignee"), pri,
		)
		if err != nil {
			return "", err
		}
		return fmt.Sprintf("Created issue %s", id), nil

	case "add_comment":
		return "Comment added", ex.ExecAddComment(str("issue_id"), str("body"))

	case "add_to_cycle":
		ids := toStringSlice(args["issue_ids"])
		return fmt.Sprintf("Added %d issue(s) to cycle", len(ids)),
			ex.ExecAddToCycle(str("cycle"), ids)

	case "start_cycle":
		return "Cycle started", ex.ExecStartCycle(str("cycle"))

	case "archive_issue":
		return "Issue archived", ex.ExecArchiveIssue(str("issue_id"))

	case "navigate":
		target := str("target")
		lower := strings.ToLower(target)
		switch {
		case lower == "all issues":
			ex.NavToAllIssues()
		case lower == "my issues":
			ex.NavFilterMyIssues(true)
		default:
			// Try project first, then cycle
			ex.NavToProject(target)
			ex.NavToCycle(target)
		}
		return fmt.Sprintf("Navigated to: %s", target), nil

	case "show_overlay":
		ex.NavOpenOverlay(str("panel"))
		return fmt.Sprintf("Opened %s", str("panel")), nil

	case "list_issues":
		issues := ex.ReadCurrentIssues()
		b, _ := json.Marshal(issues)
		return string(b), nil

	case "list_cycles":
		cycles := ex.ReadCycles()
		b, _ := json.Marshal(cycles)
		return string(b), nil

	case "list_workflow_states":
		states := ex.ReadWorkflowStates()
		b, _ := json.Marshal(states)
		return string(b), nil

	case "list_projects":
		projects := ex.ReadProjects()
		b, _ := json.Marshal(projects)
		return string(b), nil

	case "list_users":
		users := ex.ReadUsers()
		b, _ := json.Marshal(users)
		return string(b), nil

	case "refresh":
		ex.NavRefresh()
		return "Refreshed", nil

	default:
		return "", fmt.Errorf("unknown tool: %s", tc.Function.Name)
	}
}

func toStringSlice(v interface{}) []string {
	arr, ok := v.([]interface{})
	if !ok {
		return nil
	}
	out := make([]string, 0, len(arr))
	for _, item := range arr {
		if s, ok := item.(string); ok {
			out = append(out, s)
		}
	}
	return out
}

func summarizeArgs(args map[string]interface{}) string {
	parts := make([]byte, 0, 64)
	for k, v := range args {
		if len(parts) > 0 {
			parts = append(parts, ',', ' ')
		}
		parts = append(parts, []byte(fmt.Sprintf("%s=%v", k, v))...)
		if len(parts) > 60 {
			parts = append(parts, []byte("...")...)
			break
		}
	}
	return string(parts)
}
