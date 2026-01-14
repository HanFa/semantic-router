// Package anthropic provides a client for the Anthropic Claude API that accepts
// OpenAI-format requests and returns OpenAI-format responses.
package anthropic

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"time"

	"github.com/anthropics/anthropic-sdk-go"
	"github.com/anthropics/anthropic-sdk-go/option"
	"github.com/openai/openai-go"
)

// DefaultMaxTokens is the default max tokens if not specified in request
const DefaultMaxTokens int64 = 4096

// Client wraps the Anthropic SDK and provides OpenAI-compatible interface
type Client struct {
	sdk anthropic.Client
}

// NewClient creates a new Anthropic client
func NewClient(apiKey string) *Client {
	return &Client{
		sdk: anthropic.NewClient(option.WithAPIKey(apiKey)),
	}
}

// ChatCompletion processes an OpenAI-format request and returns an OpenAI-format response
func (c *Client) ChatCompletion(ctx context.Context, req *openai.ChatCompletionNewParams) ([]byte, error) {
	// Convert and call Anthropic API
	anthropicReq := c.toAnthropicRequest(req)

	resp, err := c.sdk.Messages.New(ctx, anthropicReq)
	if err != nil {
		return nil, fmt.Errorf("anthropic API error: %w", err)
	}

	// Convert response to OpenAI format and serialize
	return json.Marshal(c.toOpenAIResponse(resp, req.Model))
}

// toAnthropicRequest converts OpenAI request to Anthropic format
func (c *Client) toAnthropicRequest(req *openai.ChatCompletionNewParams) anthropic.MessageNewParams {
	var messages []anthropic.MessageParam
	var systemPrompt string

	// Process messages - extract system prompt separately (Anthropic requirement)
	for _, msg := range req.Messages {
		switch {
		case msg.OfSystem != nil:
			systemPrompt = c.extractSystemContent(msg.OfSystem)
		case msg.OfUser != nil:
			content := c.extractUserContent(msg.OfUser)
			messages = append(messages, anthropic.NewUserMessage(anthropic.NewTextBlock(content)))
		case msg.OfAssistant != nil:
			content := c.extractAssistantContent(msg.OfAssistant)
			messages = append(messages, anthropic.NewAssistantMessage(anthropic.NewTextBlock(content)))
		}
	}

	// Determine max tokens (required for Anthropic)
	maxTokens := DefaultMaxTokens
	if req.MaxCompletionTokens.Value > 0 {
		maxTokens = req.MaxCompletionTokens.Value
	} else if req.MaxTokens.Value > 0 {
		maxTokens = req.MaxTokens.Value
	}

	params := anthropic.MessageNewParams{
		Model:     anthropic.Model(req.Model),
		Messages:  messages,
		MaxTokens: maxTokens,
	}

	// Set system prompt if present
	if systemPrompt != "" {
		params.System = []anthropic.TextBlockParam{
			{Text: systemPrompt},
		}
	}

	// Set optional parameters
	if req.Temperature.Value > 0 {
		params.Temperature = anthropic.Float(req.Temperature.Value)
	}
	if req.TopP.Value > 0 {
		params.TopP = anthropic.Float(req.TopP.Value)
	}
	if len(req.Stop.OfStringArray) > 0 {
		params.StopSequences = req.Stop.OfStringArray
	} else if req.Stop.OfString.Value != "" {
		params.StopSequences = []string{req.Stop.OfString.Value}
	}

	return params
}

// extractSystemContent extracts text from a system message
func (c *Client) extractSystemContent(msg *openai.ChatCompletionSystemMessageParam) string {
	if msg.Content.OfString.Value != "" {
		return msg.Content.OfString.Value
	}
	var parts []string
	for _, part := range msg.Content.OfArrayOfContentParts {
		if part.Text != "" {
			parts = append(parts, part.Text)
		}
	}
	return strings.Join(parts, " ")
}

// extractUserContent extracts text from a user message
func (c *Client) extractUserContent(msg *openai.ChatCompletionUserMessageParam) string {
	if msg.Content.OfString.Value != "" {
		return msg.Content.OfString.Value
	}
	var parts []string
	for _, part := range msg.Content.OfArrayOfContentParts {
		if part.OfText != nil {
			parts = append(parts, part.OfText.Text)
		}
	}
	return strings.Join(parts, " ")
}

// extractAssistantContent extracts text from an assistant message
func (c *Client) extractAssistantContent(msg *openai.ChatCompletionAssistantMessageParam) string {
	if msg.Content.OfString.Value != "" {
		return msg.Content.OfString.Value
	}
	var parts []string
	for _, part := range msg.Content.OfArrayOfContentParts {
		if part.OfText != nil {
			parts = append(parts, part.OfText.Text)
		}
	}
	return strings.Join(parts, " ")
}

// OpenAI response types for serialization
type openAIResponse struct {
	ID      string         `json:"id"`
	Object  string         `json:"object"`
	Created int64          `json:"created"`
	Model   string         `json:"model"`
	Choices []openAIChoice `json:"choices"`
	Usage   openAIUsage    `json:"usage"`
}

type openAIChoice struct {
	Index        int           `json:"index"`
	Message      openAIMessage `json:"message"`
	FinishReason string        `json:"finish_reason"`
}

type openAIMessage struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type openAIUsage struct {
	PromptTokens     int `json:"prompt_tokens"`
	CompletionTokens int `json:"completion_tokens"`
	TotalTokens      int `json:"total_tokens"`
}

// toOpenAIResponse converts Anthropic response to OpenAI format
func (c *Client) toOpenAIResponse(resp *anthropic.Message, model string) *openAIResponse {
	// Extract text content
	var content string
	for _, block := range resp.Content {
		if block.Type == "text" {
			content += block.Text
		}
	}

	// Map stop reason
	finishReason := "stop"
	switch resp.StopReason {
	case anthropic.StopReasonMaxTokens:
		finishReason = "length"
	case anthropic.StopReasonToolUse:
		finishReason = "tool_calls"
	}

	return &openAIResponse{
		ID:      resp.ID,
		Object:  "chat.completion",
		Created: time.Now().Unix(),
		Model:   model,
		Choices: []openAIChoice{{
			Index:        0,
			Message:      openAIMessage{Role: "assistant", Content: content},
			FinishReason: finishReason,
		}},
		Usage: openAIUsage{
			PromptTokens:     int(resp.Usage.InputTokens),
			CompletionTokens: int(resp.Usage.OutputTokens),
			TotalTokens:      int(resp.Usage.InputTokens + resp.Usage.OutputTokens),
		},
	}
}
