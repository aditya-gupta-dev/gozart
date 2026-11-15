package config

import (
	"encoding/json"
	"os"
	"path/filepath"
	"testing"

	"gozart/pkg/logger"
)

func TestConfigDefaultsAndBridging(t *testing.T) {
	tmpDir := t.TempDir()
	origDir, _ := os.Getwd()
	_ = os.Chdir(tmpDir)
	defer func() { _ = os.Chdir(origDir) }()

	l := logger.New()
	defer l.Close()

	// 1. Test defaults when config.json does not exist
	loader := New(l)
	if loader.ConfigData.Server.Port != 8080 {
		t.Errorf("expected default port 8080, got %d", loader.ConfigData.Server.Port)
	}
	if loader.ConfigData.AI.AgentModel != "qwen2.5:32b" {
		t.Errorf("expected default agent model qwen2.5:32b, got %s", loader.ConfigData.AI.AgentModel)
	}
	if loader.ConfigData.Storage.OutputDir != "output" {
		t.Errorf("expected default output dir output, got %s", loader.ConfigData.Storage.OutputDir)
	}
	if loader.ConfigData.OutputDirectory != "output" {
		t.Errorf("expected bridged legacy OutputDirectory to be output, got %s", loader.ConfigData.OutputDirectory)
	}

	// 2. Test legacy bridging from JSON
	legacyJSON := `{
		"youtube-api-key": "AIzaSyLegacyKey123",
		"output-directory": "custom_output"
	}`
	_ = os.WriteFile(filepath.Join(tmpDir, "config.json"), []byte(legacyJSON), 0644)

	loader2 := New(l)
	if loader2.ConfigData.YouTube.APIKey != "AIzaSyLegacyKey123" {
		t.Errorf("expected bridged YouTube.APIKey to be AIzaSyLegacyKey123, got %s", loader2.ConfigData.YouTube.APIKey)
	}
	if loader2.ConfigData.Storage.OutputDir != "custom_output" {
		t.Errorf("expected bridged Storage.OutputDir to be custom_output, got %s", loader2.ConfigData.Storage.OutputDir)
	}

	// 3. Test SaveConfig
	loader2.ConfigData.Server.Port = 9090
	loader2.ConfigData.AI.AgentModel = "custom-agent-v1"
	if err := loader2.SaveConfig(); err != nil {
		t.Fatalf("SaveConfig failed: %v", err)
	}

	savedBytes, err := os.ReadFile(filepath.Join(tmpDir, "config.json"))
	if err != nil {
		t.Fatalf("failed to read saved config.json: %v", err)
	}

	var parsed ConfigParams
	if err := json.Unmarshal(savedBytes, &parsed); err != nil {
		t.Fatalf("failed to parse saved config.json: %v", err)
	}

	if parsed.Server.Port != 9090 {
		t.Errorf("expected saved port 9090, got %d", parsed.Server.Port)
	}
	if parsed.AI.AgentModel != "custom-agent-v1" {
		t.Errorf("expected saved agent model custom-agent-v1, got %s", parsed.AI.AgentModel)
	}
}
