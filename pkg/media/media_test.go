package media

import (
	"context"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
	"time"

	"gozart/pkg/agent"
)

func TestSelfHealer(t *testing.T) {
	tmpDir := t.TempDir()
	propsPath := filepath.Join(tmpDir, "props.json")

	// Create malformed props with negative duration and invalid format
	initialProps := map[string]interface{}{
		"duration_sec": -10.0,
		"format":       "unsupported_3d_vr",
		"scenes": []interface{}{
			map[string]interface{}{
				"scene_index": 1,
				"voiceover":   "This is a \"quoted\" sentence.",
			},
		},
	}
	data, _ := json.Marshal(initialProps)
	if err := os.WriteFile(propsPath, data, 0644); err != nil {
		t.Fatalf("failed to write props: %v", err)
	}

	healer := NewSelfHealer(3)
	retryCount := 0
	healed, err := healer.AttemptHealing(context.Background(), "duration negative", propsPath, func() error {
		retryCount++
		return nil
	})

	if err != nil {
		t.Fatalf("AttemptHealing failed: %v", err)
	}
	if !healed {
		t.Fatalf("expected healed to be true")
	}
	if retryCount != 1 {
		t.Errorf("expected 1 retry call, got %d", retryCount)
	}

	// Verify repaired props file
	repairedBytes, _ := os.ReadFile(propsPath)
	var repaired map[string]interface{}
	_ = json.Unmarshal(repairedBytes, &repaired)

	if dur, ok := repaired["duration_sec"].(float64); !ok || dur <= 0 {
		t.Errorf("expected positive duration after healing, got %v", repaired["duration_sec"])
	}
	if fmtStr, ok := repaired["format"].(string); !ok || fmtStr != "shorts_9_16" {
		t.Errorf("expected format shorts_9_16 after healing, got %v", repaired["format"])
	}
}

func TestRemotionRendererKineticFallback(t *testing.T) {
	tmpDir := t.TempDir()
	workDir := filepath.Join(tmpDir, "work")
	outVideo := filepath.Join(tmpDir, "visuals.mp4")

	renderer := NewRemotion("node", "ffmpeg", 2)

	script := &agent.Script{
		TitleIdea: "Speed of Light",
		Hook: agent.Hook{
			Formula:         "Curiosity",
			Voiceover:       "Nothing travels faster than light.",
			VisualDirection: "Cosmic rays",
			DurationSec:     1,
		},
		Scenes: []agent.Scene{
			{
				SceneID:              1,
				Type:                 "data_revelation",
				Voiceover:            "At 300,000 kilometers per second.",
				VisualDescription:    "Photons in vacuum",
				AnimationSnippetType: "kinetic_typography",
				DurationSec:          2,
			},
		},
		CTA: agent.CTA{
			Voiceover:         "Subscribe for more science.",
			VisualDescription: "Subscribe button",
			DurationSec:       1,
		},
		TotalEstimatedDuration: 2,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()

	if err := renderer.RenderVisuals(ctx, script, "shorts_9_16", workDir, outVideo); err != nil {
		t.Fatalf("RenderVisuals failed: %v", err)
	}

	info, err := os.Stat(outVideo)
	if err != nil {
		t.Fatalf("visuals.mp4 does not exist: %v", err)
	}
	if info.Size() == 0 {
		t.Errorf("visuals.mp4 is empty")
	}
}
