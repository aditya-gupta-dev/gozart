package agent

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
)

type Hook struct {
	Formula         string  `json:"formula"`
	Voiceover       string  `json:"voiceover"`
	VisualDirection string  `json:"visual_direction"`
	DurationSec     float64 `json:"duration_sec"`
}

type Scene struct {
	SceneID              int     `json:"scene_id"`
	Type                 string  `json:"type"`
	Voiceover            string  `json:"voiceover"`
	VisualDescription    string  `json:"visual_description"`
	AnimationSnippetType string  `json:"animation_snippet_type"`
	SFXCue               string  `json:"sfx_cue"`
	DurationSec          float64 `json:"duration_sec"`
}

type CTA struct {
	Voiceover         string  `json:"voiceover"`
	VisualDescription string  `json:"visual_description"`
	DurationSec       float64 `json:"duration_sec"`
}

type Script struct {
	TitleIdea              string  `json:"title_idea"`
	TotalEstimatedDuration float64 `json:"total_estimated_duration"`
	Hook                   Hook    `json:"hook"`
	Scenes                 []Scene `json:"scenes"`
	CTA                    CTA     `json:"cta"`
}

// FullVoiceoverText concatenates Hook, Scenes, and CTA voiceover.
func (s *Script) FullVoiceoverText() string {
	var parts []string
	if s.Hook.Voiceover != "" {
		parts = append(parts, s.Hook.Voiceover)
	}
	for _, scene := range s.Scenes {
		if scene.Voiceover != "" {
			parts = append(parts, scene.Voiceover)
		}
	}
	if s.CTA.Voiceover != "" {
		parts = append(parts, s.CTA.Voiceover)
	}
	return strings.Join(parts, " ")
}

// WriteScript orchestrates script synthesis using the LLM agent.
func WriteScript(
	ctx context.Context,
	harness *AgentHarness,
	topic string,
	countryCode string,
	format string,
	tone string,
	targetDurationSec int,
	transcriptSummaries string,
	sentimentReport string,
) (*Script, error) {
	if tone == "" {
		tone = "sensational"
	}
	if format == "" {
		format = "shorts_9_16"
	}
	if targetDurationSec <= 0 {
		targetDurationSec = 50
	}

	userPrompt := fmt.Sprintf(`Generate a viral YouTube script.
Topic: %s
Target Country: %s
Format: %s
Tone: %s
Target Duration: %d seconds

Competitive Video Transcripts:
%s

Audience Sentiment & Curiosity Gaps:
%s`, topic, countryCode, format, tone, targetDurationSec, transcriptSummaries, sentimentReport)

	var script Script
	if harness != nil {
		jsonStr, err := harness.GenerateStructuredJSON(ctx, "", ScriptwritingSystemPrompt, userPrompt)
		if err == nil {
			if err := json.Unmarshal([]byte(jsonStr), &script); err == nil && len(script.Scenes) > 0 {
				return &script, nil
			}
		}
	}

	// Fallback script if LLM is offline or returned unparseable output
	fallback := GenerateFallbackScript(topic, format, targetDurationSec)
	return fallback, nil
}

// GenerateFallbackScript creates a well-structured script if LLM is unavailable.
func GenerateFallbackScript(topic string, format string, targetDurationSec int) *Script {
	return &Script{
		TitleIdea:              fmt.Sprintf("The Untold Truth About %s", topic),
		TotalEstimatedDuration: float64(targetDurationSec),
		Hook: Hook{
			Formula:         "Negative Inversion + Curiosity Gap",
			Voiceover:       fmt.Sprintf("Everyone thinks they know everything about %s, but almost everyone has the story completely wrong.", topic),
			VisualDirection: "Fast camera push-in with high-contrast text overlay and glitch transition",
			DurationSec:     5.0,
		},
		Scenes: []Scene{
			{
				SceneID:              1,
				Type:                 "kinetic_typography_with_data",
				Voiceover:            fmt.Sprintf("Behind the scenes of %s lies an unexpected fact that changed everything.", topic),
				VisualDescription:    "Split-screen comparison showing timeline progression and dynamic stat counter",
				AnimationSnippetType: "SplitCounter",
				SFXCue:               "ticking_clock_heavy_riser",
				DurationSec:          6.0,
			},
			{
				SceneID:              2,
				Type:                 "chart_comparison",
				Voiceover:            "When you analyze the data, the conventional narrative completely falls apart.",
				VisualDescription:    "Animated downward trending chart highlighting key discrepancy",
				AnimationSnippetType: "BarChartDrop",
				SFXCue:               "bass_drop",
				DurationSec:          7.0,
			},
			{
				SceneID:              3,
				Type:                 "curiosity_reveal",
				Voiceover:            "The real reason was kept hidden in plain sight until recent records came to light.",
				VisualDescription:    "Dramatic zoom on archival document with glowing highlighted text",
				AnimationSnippetType: "ZoomHighlight",
				SFXCue:               "whoosh_riser",
				DurationSec:          6.0,
			},
		},
		CTA: CTA{
			Voiceover:         "Do you agree or is there more to the story? Let me know in the comments below.",
			VisualDescription: "Interactive voting graphic with animated pulsing indicator",
			DurationSec:       4.0,
		},
	}
}
