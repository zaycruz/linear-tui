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
	anthropicEndpoint = "https://api.anthropic.com/v1/messages"
	anthropicVersion  = "2023-06-01"
	chatModel         = "claude-sonnet-4-6"
)

// ChatMessage is a single turn in a chat conversation.
type ChatMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

// ChatClient streams completions from the Anthropic API.
type ChatClient struct {
	apiKey string
	http   *http.Client
}

// NewChatClient reads ANTHROPIC_API_KEY from the environment.
func NewChatClient() *ChatClient {
	return &ChatClient{
		apiKey: os.Getenv("ANTHROPIC_API_KEY"),
		http:   &http.Client{},
	}
}

// Available reports whether an API key is configured.
func (c *ChatClient) Available() bool {
	return c.apiKey != ""
}

// Stream sends history to the API and calls onToken for each text delta.
// onDone is called once with the full response text (or an error).
// The call is non-blocking — execution happens in a goroutine.
func (c *ChatClient) Stream(ctx context.Context, system string, history []ChatMessage, onToken func(string), onDone func(string, error)) {
	go func() {
		payload, _ := json.Marshal(map[string]interface{}{
			"model":      chatModel,
			"max_tokens": 1024,
			"system":     system,
			"messages":   history,
			"stream":     true,
		})

		req, err := http.NewRequestWithContext(ctx, http.MethodPost, anthropicEndpoint, bytes.NewReader(payload))
		if err != nil {
			onDone("", err)
			return
		}
		req.Header.Set("x-api-key", c.apiKey)
		req.Header.Set("anthropic-version", anthropicVersion)
		req.Header.Set("content-type", "application/json")

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
				Type  string `json:"type"`
				Delta struct {
					Type string `json:"type"`
					Text string `json:"text"`
				} `json:"delta"`
			}
			if json.Unmarshal([]byte(data), &ev) != nil {
				continue
			}
			if ev.Type == "content_block_delta" && ev.Delta.Type == "text_delta" {
				onToken(ev.Delta.Text)
				full.WriteString(ev.Delta.Text)
			}
		}
		onDone(full.String(), scanner.Err())
	}()
}
