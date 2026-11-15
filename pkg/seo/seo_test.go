package seo

import (
	"context"
	"testing"

	"gozart/pkg/agent"
)

func TestSEOFallback(t *testing.T) {
	script := &agent.Script{
		TitleIdea: "The Mystery of Saturn",
		Scenes: []agent.Scene{
			{SceneID: 1, Type: "intro", DurationSec: 5.0},
			{SceneID: 2, Type: "rings", DurationSec: 8.0},
		},
	}

	meta, err := GenerateSEO(context.Background(), nil, "Saturn", "US", script, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(meta.TitleCandidates) < 3 {
		t.Errorf("expected at least 3 title candidates, got %d", len(meta.TitleCandidates))
	}
	if len(meta.Tags) < 10 {
		t.Errorf("expected at least 10 tags, got %d", len(meta.Tags))
	}
	if meta.ChosenTitle == "" {
		t.Errorf("expected non-empty chosen title")
	}
}
