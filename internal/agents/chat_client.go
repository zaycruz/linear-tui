package agents

import (
	"bufio"
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"os"
	"strings"
)

const (
	openRouterEndpoint = "https://openrouter.ai/api/v1/chat/completions"
	chatModel          = "minimax/minimax-m1-2.7"
)

// ChatMessage is a single turn in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatClient streams completions from OpenRouter.
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

// Stream sends history to OpenRouter and calls onToken for each text delta.
// System prompt is prepended as the first message with role "system".
// onDone is called once with the full response text (or an error).
// The call is non-blocking — execution happens in a goroutine.
func (c *ChatClient) Stream(ctx context.Context, system string, history []ChatMessage, onToken func(string), onDone func(string, error)) {
	go func() {
		msgs := make([]ChatMessage, 0, len(history)+1)
		if system != "" {
			msgs = append(msgs, ChatMessage{Role: "system", Content: system})
		}
		msgs = append(msgs, history...)

		payload, _ := json.Marshal(map[string]interface{}{
			"model":    chatModel,
			"messages": msgs,
			"stream":   true,
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, openRouterEndpoint, bytes.NewReader(payload))
		if err != nil {
			onDone("", err)
			return
		}
		req.Header.Set("Authorization", "Bearer "+c.apiKey)
		req.Header.Set("Content-Type", "application/json")
		req.Header.Set("HTTP-Referer", "https://github.com/zaycruz/linear-tui")
		req.Header.Set("X-Title", "linear-tui")

		resp, err := c.http.Do(req)
		if err != nil {
			onDone("", err)
			return
		}
		defer resp.Body.Close()

		if resp.StatusCode != http.StatusOK {
			onDone("", fmt.Errorf("API %s", resp.Status))
			return
		}

		var full strings.Builder
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
			var ev struct {
				Choices []struct {
					Delta struct {
						Content string `json:"content"`
					} `json:"delta"`
				} `json:"choices"`
			}
			if json.Unmarshal([]byte(data), &ev) != nil {
				continue
			}
			if len(ev.Choices) > 0 {
				token := ev.Choices[0].Delta.Content
				if token != "" {
					onToken(token)
					full.WriteString(token)
				}
			}
		}
		onDone(full.String(), scanner.Err())
	}()
}
