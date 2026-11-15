package agent

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"regexp"
	"strings"
	"time"

	"gozart/pkg/config"
	"gozart/pkg/research"

	"github.com/ollama/ollama/api"
)

type AgentHarness struct {
	client     *api.Client
	config     config.AIConfig
	researcher *research.YouTubeResearcher
	modelTier  string
	httpClient *http.Client
}

func New(cfg config.AIConfig, researcher *research.YouTubeResearcher) *AgentHarness {
	baseURL, err := url.Parse(cfg.OllamaBaseURL)
	if err != nil {
		baseURL, _ = url.Parse("http://localhost:11434")
	}

	httpClient := &http.Client{
		Timeout: 180 * time.Second,
	}

	client := api.NewClient(baseURL, httpClient)
	return &AgentHarness{
		client:     client,
		config:     cfg,
		researcher: researcher,
		modelTier:  "local",
		httpClient: httpClient,
	}
}

// SetModelTier switches between "local" (Ollama) and "cloud" (OpenAI-compatible) execution.
func (a *AgentHarness) SetModelTier(tier string) {
	if tier != "" {
		a.modelTier = strings.ToLower(tier)
	}
}

// ChatWithTools executes an LLM agent loop supporting tool calling.
func (a *AgentHarness) ChatWithTools(ctx context.Context, model string, systemPrompt, userPrompt string) (string, error) {
	if model == "" {
		model = a.config.AgentModel
	}

	tool := GetSearchYouTubeVideosTool()
	stream := false

	messages := []api.Message{
		{Role: "system", Content: systemPrompt},
		{Role: "user", Content: userPrompt},
	}

	req := &api.ChatRequest{
		Model:    model,
		Messages: messages,
		Stream:   &stream,
		Tools:    api.Tools{tool},
	}

	var finalResponse string
	err := a.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		if len(resp.Message.ToolCalls) > 0 {
			messages = append(messages, resp.Message)
			for _, tc := range resp.Message.ToolCalls {
				if tc.Function.Name == "search_youtube_videos" {
					argsData, _ := json.Marshal(tc.Function.Arguments)
					toolResult, toolErr := ExecuteSearchYouTubeVideos(ctx, a.researcher, string(argsData))
					if toolErr != nil {
						toolResult = fmt.Sprintf(`{"error": "%s"}`, toolErr.Error())
					}
					messages = append(messages, api.Message{
						Role:    "tool",
						Content: toolResult,
					})
				}
			}

			// Follow up with tool results
			followUpReq := &api.ChatRequest{
				Model:    model,
				Messages: messages,
				Stream:   &stream,
			}
			return a.client.Chat(ctx, followUpReq, func(followUpResp api.ChatResponse) error {
				finalResponse = followUpResp.Message.Content
				return nil
			})
		}

		finalResponse = resp.Message.Content
		return nil
	})

	if err != nil {
		return "", fmt.Errorf("ollama chat error: %w", err)
	}

	return finalResponse, nil
}

// GenerateStructuredJSON requests JSON format from the model and cleans backticks.
func (a *AgentHarness) GenerateStructuredJSON(ctx context.Context, model string, systemPrompt, userPrompt string) (string, error) {
	if a.modelTier == "cloud" || strings.EqualFold(a.config.Provider, "openai") || strings.EqualFold(a.config.Provider, "cloud") {
		return a.generateStructuredJSONCloud(ctx, model, systemPrompt, userPrompt)
	}

	if model == "" {
		model = a.config.FastModel
	}

	stream := false
	jsonFormat := json.RawMessage(`"json"`)

	req := &api.ChatRequest{
		Model: model,
		Messages: []api.Message{
			{Role: "system", Content: systemPrompt},
			{Role: "user", Content: userPrompt},
		},
		Format: jsonFormat,
		Stream: &stream,
	}

	var responseText string
	err := a.client.Chat(ctx, req, func(resp api.ChatResponse) error {
		responseText = resp.Message.Content
		return nil
	})

	if err != nil {
		// If Ollama fails and an API key is configured, fallback to cloud
		if a.config.APIKey != "" || os.Getenv("OPENAI_API_KEY") != "" {
			cloudRes, cloudErr := a.generateStructuredJSONCloud(ctx, model, systemPrompt, userPrompt)
			if cloudErr == nil && cloudRes != "" {
				return cloudRes, nil
			}
		}
		return "", fmt.Errorf("ollama json generation failed: %w", err)
	}

	cleaned := CleanJSONResponse(responseText)
	return cleaned, nil
}

func (a *AgentHarness) generateStructuredJSONCloud(ctx context.Context, model string, systemPrompt, userPrompt string) (string, error) {
	apiKey := a.config.APIKey
	if apiKey == "" {
		apiKey = os.Getenv("OPENAI_API_KEY")
	}

	if model == "" || strings.HasPrefix(model, "qwen") || strings.HasPrefix(model, "llama") {
		model = "gpt-4o-mini"
	}

	endpoint := "https://api.openai.com/v1/chat/completions"
	if a.config.Provider == "openai" && a.config.OllamaBaseURL != "" && !strings.Contains(a.config.OllamaBaseURL, "11434") {
		endpoint = strings.TrimSuffix(a.config.OllamaBaseURL, "/") + "/chat/completions"
	}

	reqPayload := map[string]interface{}{
		"model": model,
		"messages": []map[string]string{
			{"role": "system", "content": systemPrompt},
			{"role": "user", "content": userPrompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	body, err := json.Marshal(reqPayload)
	if err != nil {
		return "", err
	}

	httpReq, err := http.NewRequestWithContext(ctx, "POST", endpoint, bytes.NewReader(body))
	if err != nil {
		return "", err
	}
	httpReq.Header.Set("Content-Type", "application/json")
	if apiKey != "" {
		httpReq.Header.Set("Authorization", "Bearer "+apiKey)
	}

	resp, err := a.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("cloud llm request failed: %w", err)
	}
	defer resp.Body.Close()

	respBytes, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", err
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("cloud llm returned status %d: %s", resp.StatusCode, string(respBytes))
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := json.Unmarshal(respBytes, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices from cloud llm")
	}

	return CleanJSONResponse(chatResp.Choices[0].Message.Content), nil
}

var codeBlockRegex = regexp.MustCompile("(?s)```(?:json)?\\s*(.*?)\\s*```")

// CleanJSONResponse strips markdown code fences if present.
func CleanJSONResponse(raw string) string {
	raw = strings.TrimSpace(raw)
	matches := codeBlockRegex.FindStringSubmatch(raw)
	if len(matches) > 1 {
		return strings.TrimSpace(matches[1])
	}
	return raw
}
