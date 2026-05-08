package agents

import (
	"bufio"
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

// streamChunk is one SSE delta from the OpenRouter streaming API.
type streamChunk struct {
	Choices []struct {
		Delta struct {
			Role      string  `json:"role"`
			Content   *string `json:"content"`
			ToolCalls []struct {
				Index    int    `json:"index"`
				ID       string `json:"id"`
				Type     string `json:"type"`
				Function struct {
					Name      string `json:"name"`
					Arguments string `json:"arguments"`
				} `json:"function"`
			} `json:"tool_calls"`
		} `json:"delta"`
		FinishReason *string `json:"finish_reason"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

// turnResult is the assembled output of one streaming turn.
type turnResult struct {
	content      string
	toolCalls    []ToolCall
	finishReason string
}

// completeStream sends one turn to the API with streaming and returns the assembled result.
// onToken is called for each text token as it arrives (may be nil for tool-call turns).
func (c *ChatClient) completeStream(ctx context.Context, messages []ChatMessage, useTools bool, onToken func(string)) (*turnResult, error) {
	body := map[string]interface{}{
		"model":    chatModel,
		"messages": messages,
		"stream":   true,
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

	if resp.StatusCode != http.StatusOK {
		raw, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("API %s: %s", resp.Status, raw)
	}

	var contentBuf strings.Builder
	partialTCs := map[int]*ToolCall{}
	finishReason := ""

	scanner := bufio.NewScanner(resp.Body)
	for scanner.Scan() {
		line := scanner.Text()
		if !strings.HasPrefix(line, "data: ") {
			continue
		}
		data := strings.TrimPrefix(line, "data: ")
		if data == "[DONE]" {
			break
		}

		var chunk streamChunk
		if err := json.Unmarshal([]byte(data), &chunk); err != nil {
			continue
		}
		if chunk.Error != nil {
			return nil, fmt.Errorf("API error: %s", chunk.Error.Message)
		}
		if len(chunk.Choices) == 0 {
			continue
		}

		delta := chunk.Choices[0].Delta

		if delta.Content != nil && *delta.Content != "" {
			contentBuf.WriteString(*delta.Content)
			if onToken != nil {
				onToken(*delta.Content)
			}
		}

		for _, tc := range delta.ToolCalls {
			if _, ok := partialTCs[tc.Index]; !ok {
				partialTCs[tc.Index] = &ToolCall{ID: tc.ID, Type: tc.Type}
			}
			p := partialTCs[tc.Index]
			if tc.ID != "" {
				p.ID = tc.ID
			}
			if tc.Function.Name != "" {
				p.Function.Name = tc.Function.Name
			}
			p.Function.Arguments += tc.Function.Arguments
		}

		if chunk.Choices[0].FinishReason != nil {
			finishReason = *chunk.Choices[0].FinishReason
		}
	}

	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("stream read error: %w", err)
	}

	toolCalls := make([]ToolCall, 0, len(partialTCs))
	for i := 0; i < len(partialTCs); i++ {
		if tc, ok := partialTCs[i]; ok {
			toolCalls = append(toolCalls, *tc)
		}
	}

	return &turnResult{
		content:      contentBuf.String(),
		toolCalls:    toolCalls,
		finishReason: finishReason,
	}, nil
}

// RunAgent drives the full agentic loop.
// onStatus is called for tool-call status lines shown in the chat pane.
// onToken is called for each streamed text token of the final response.
// onDone is called when the loop ends; reply contains the full response text (for history).
// The call is non-blocking — runs in a goroutine.
func (c *ChatClient) RunAgent(
	ctx context.Context,
	system string,
	history []ChatMessage,
	executor ToolExecutor,
	onStatus func(string),
	onToken func(string),
	onDone func(string, error),
) {
	go func() {
		msgs := make([]ChatMessage, 0, len(history)+1)
		if system != "" {
			msgs = append(msgs, ChatMessage{Role: "system", Content: system})
		}
		msgs = append(msgs, history...)

		for i := 0; i < maxAgentIterations; i++ {
			result, err := c.completeStream(ctx, msgs, true, onToken)
			if err != nil {
				onDone("", err)
				return
			}

			// Final text response
			if result.finishReason == "stop" || len(result.toolCalls) == 0 {
				onDone(result.content, nil)
				return
			}

			// Append assistant message with tool calls
			msgs = append(msgs, ChatMessage{
				Role:      "assistant",
				Content:   nil,
				ToolCalls: result.toolCalls,
			})

			// Execute each tool call
			for _, tc := range result.toolCalls {
				args := tc.Args()
				onStatus(fmt.Sprintf("→ %s(%s)", tc.Function.Name, summarizeArgs(args)))

				toolResult, execErr := executeToolCall(ctx, tc, executor)
				if execErr != nil {
					toolResult = fmt.Sprintf("error: %s", execErr.Error())
				}

				msgs = append(msgs, ChatMessage{
					Role:       "tool",
					Content:    toolResult,
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
