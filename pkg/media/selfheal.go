package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"strings"
)

type SelfHealer struct {
	MaxRetries int
}

func NewSelfHealer(maxRetries int) *SelfHealer {
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &SelfHealer{MaxRetries: maxRetries}
}

// AttemptHealing analyzes rendering errors, fixes props or syntax, and retries rendering.
func (h *SelfHealer) AttemptHealing(ctx context.Context, errorMsg string, propsPath string, retryFunc func() error) (bool, error) {
	for attempt := 1; attempt <= h.MaxRetries; attempt++ {
		// Read current props
		data, err := os.ReadFile(propsPath)
		if err != nil {
			return false, err
		}

		var props map[string]interface{}
		if err := json.Unmarshal(data, &props); err != nil {
			return false, fmt.Errorf("malformed props json: %w", err)
		}

		// Self-healing rules:
		// 1. Missing duration or negative duration
		if dur, ok := props["duration_sec"].(float64); !ok || dur <= 0 {
			props["duration_sec"] = 30.0
		}

		// 2. Format sanitization
		if fmtStr, ok := props["format"].(string); !ok || (fmtStr != "shorts_9_16" && fmtStr != "longform_16_9") {
			props["format"] = "shorts_9_16"
		}

		// 3. String escaping issues in scenes
		if scenes, ok := props["scenes"].([]interface{}); ok {
			for _, s := range scenes {
				if sceneMap, ok := s.(map[string]interface{}); ok {
					if vo, ok := sceneMap["voiceover"].(string); ok {
						sceneMap["voiceover"] = strings.ReplaceAll(vo, "\"", "'")
					}
				}
			}
		}

		repairedData, err := json.MarshalIndent(props, "", "  ")
		if err == nil {
			_ = os.WriteFile(propsPath, repairedData, 0644)
		}

		// Retry rendering
		if retryErr := retryFunc(); retryErr == nil {
			return true, nil
		}
	}

	return false, fmt.Errorf("self-healing exhausted after %d retries for error: %s", h.MaxRetries, errorMsg)
}
