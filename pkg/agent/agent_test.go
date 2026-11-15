package agent

import (
	"strings"
	"testing"

	"gozart/pkg/config"
)

func TestCleanJSONResponse(t *testing.T) {
	testCases := []struct {
		input    string
		expected string
	}{
		{
			input:    "```json\n{\"title\": \"test\"}\n```",
			expected: "{\"title\": \"test\"}",
		},
		{
			input:    "```\n{\"title\": \"test\"}\n```",
			expected: "{\"title\": \"test\"}",
		},
		{
			input:    "{\"title\": \"test\"}",
			expected: "{\"title\": \"test\"}",
		},
		{
			input:    "   ```json\n{\"key\":\"val\"}\n```   ",
			expected: "{\"key\":\"val\"}",
		},
	}

	for _, tc := range testCases {
		got := CleanJSONResponse(tc.input)
		if got != tc.expected {
			t.Errorf("CleanJSONResponse(%q) = %q, want %q", tc.input, got, tc.expected)
		}
	}
}

func TestScriptFallbackAndVoiceover(t *testing.T) {
	script := GenerateFallbackScript("Mars Colony", "shorts_9_16", 45)
	if script == nil {
		t.Fatalf("expected non-nil script")
	}
	if len(script.Scenes) < 3 {
		t.Errorf("expected at least 3 scenes, got %d", len(script.Scenes))
	}
	if script.Hook.Voiceover == "" {
		t.Errorf("expected non-empty hook voiceover")
	}
	if script.CTA.Voiceover == "" {
		t.Errorf("expected non-empty cta voiceover")
	}

	fullText := script.FullVoiceoverText()
	if !strings.Contains(fullText, script.Hook.Voiceover) {
		t.Errorf("full text missing hook")
	}
	if !strings.Contains(fullText, script.CTA.Voiceover) {
		t.Errorf("full text missing CTA")
	}
}

func TestSearchToolDeclaration(t *testing.T) {
	tool := GetSearchYouTubeVideosTool()
	if tool.Function.Name != "search_youtube_videos" {
		t.Errorf("expected tool name search_youtube_videos, got %s", tool.Function.Name)
	}
	if len(tool.Function.Parameters.Required) != 2 {
		t.Errorf("expected 2 required parameters, got %d", len(tool.Function.Parameters.Required))
	}
}

func TestAgentModelTier(t *testing.T) {
	harness := New(config.AIConfig{
		Provider:      "ollama",
		OllamaBaseURL: "http://localhost:11434",
		AgentModel:    "qwen2.5:32b",
		FastModel:     "qwen2.5:7b",
	}, nil)

	if harness.modelTier != "local" {
		t.Errorf("expected default model tier local, got %s", harness.modelTier)
	}

	harness.SetModelTier("cloud")
	if harness.modelTier != "cloud" {
		t.Errorf("expected model tier cloud, got %s", harness.modelTier)
	}
}
