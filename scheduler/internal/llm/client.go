package llm

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"unicode/utf8"
)

// Client is an OpenAI-compatible chat completions client.
type Client struct {
	baseURL string
	model   string
	http    *http.Client
}

// NewClient creates a Client targeting the given baseURL (e.g. "http://100.77.62.33:8080")
// using the specified model name.
func NewClient(baseURL, model string) *Client {
	return &Client{baseURL: baseURL, model: model, http: &http.Client{}}
}

type chatRequest struct {
	Model    string    `json:"model"`
	Messages []message `json:"messages"`
}

type message struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatResponse struct {
	Choices []struct {
		Message message `json:"message"`
	} `json:"choices"`
}

// Complete sends a chat completion request with the given system and user prompts
// and returns the assistant's text response.
func (c *Client) Complete(ctx context.Context, system, user string) (string, error) {
	body, err := json.Marshal(chatRequest{
		Model: c.model,
		Messages: []message{
			{Role: "system", Content: system},
			{Role: "user", Content: user},
		},
	})
	if err != nil {
		return "", fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/v1/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("llm returned %d: %s", resp.StatusCode, string(b))
	}

	var cr chatResponse
	if err := json.NewDecoder(resp.Body).Decode(&cr); err != nil {
		return "", fmt.Errorf("decode response: %w", err)
	}
	if len(cr.Choices) == 0 {
		return "", fmt.Errorf("empty choices in response")
	}
	return cr.Choices[0].Message.Content, nil
}

// Translate asks the LLM to translate text to English, returning only the translation.
func (c *Client) Translate(ctx context.Context, text string) (string, error) {
	return c.Complete(ctx,
		"Translate the following text to English. Return only the translated text, no explanations, no quotes.",
		text,
	)
}

// MaybeTranslate returns text unchanged if fewer than 20% of its runes are non-ASCII
// (i.e. the text is predominantly ASCII/Latin). Otherwise it calls Translate; on error
// it logs a warning and returns the original text.
func MaybeTranslate(ctx context.Context, c *Client, text string) string {
	if text == "" {
		return text
	}
	var nonASCII int
	for _, r := range text {
		if r > 127 {
			nonASCII++
		}
	}
	ratio := float64(nonASCII) / float64(utf8.RuneCountInString(text))
	if ratio < 0.2 {
		return text
	}
	translated, err := c.Translate(ctx, text)
	if err != nil {
		slog.Warn("llm translate failed, using original", "text", text, "error", err)
		return text
	}
	return translated
}
