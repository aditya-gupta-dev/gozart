package seo

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gozart/pkg/agent"
	"gozart/pkg/config"
	"gozart/pkg/sentiment"

	"github.com/ollama/ollama/api"
)

type SEOMetadata struct {
	TitleCandidates []string `json:"title_candidates"`
	ChosenTitle     string   `json:"chosen_title"`
	Description     string   `json:"description"`
	Tags            []string `json:"tags"`
}

// GenerateSEO creates CTR-optimized titles, search description with timestamps, and discovery tags.
func GenerateSEO(
	ctx context.Context,
	harness *agent.AgentHarness,
	topic string,
	country string,
	script *agent.Script,
	sentimentReport *sentiment.SentimentReport,
) (*SEOMetadata, error) {
	scriptJSON, _ := json.Marshal(script)
	sentimentJSON, _ := json.Marshal(sentimentReport)

	userPrompt := fmt.Sprintf("Topic: %s\nCountry: %s\n\nScript:\n%s\n\nAudience Sentiment:\n%s",
		topic, country, string(scriptJSON), string(sentimentJSON))

	if harness != nil {
		jsonStr, err := harness.GenerateStructuredJSON(ctx, "", agent.SEOGenerationPrompt, userPrompt)
		if err == nil {
			var metadata SEOMetadata
			if err := json.Unmarshal([]byte(jsonStr), &metadata); err == nil && len(metadata.TitleCandidates) > 0 {
				if metadata.ChosenTitle == "" {
					metadata.ChosenTitle = metadata.TitleCandidates[0]
				}
				return &metadata, nil
			}
		}
	}

	return GenerateFallbackSEO(topic, script), nil
}

// GenerateFallbackSEO creates comprehensive YouTube metadata when LLM is unavailable.
func GenerateFallbackSEO(topic string, script *agent.Script) *SEOMetadata {
	chosenTitle := fmt.Sprintf("Why %s Was Never What You Thought", topic)
	candidates := []string{
		fmt.Sprintf("The Secret Truth Behind %s", topic),
		fmt.Sprintf("%s Explained: The Full Breakdown", topic),
		fmt.Sprintf("Why Everything You Know About %s Is Wrong", topic),
	}
	if script != nil && script.TitleIdea != "" {
		candidates[0] = script.TitleIdea
		chosenTitle = script.TitleIdea
	}

	// Generate chapters from script scenes
	var chapters strings.Builder
	chapters.WriteString("00:00 The Hook\n")
	if script != nil {
		var curSec float64 = 4.0
		for _, s := range script.Scenes {
			minutes := int(curSec) / 60
			seconds := int(curSec) % 60
			chapters.WriteString(fmt.Sprintf("%02d:%02d Scene %d: %s\n", minutes, seconds, s.SceneID, s.Type))
			curSec += s.DurationSec
		}
	}

	cleanTopic := strings.ReplaceAll(strings.ToLower(topic), " ", "")
	desc := fmt.Sprintf("Everyone thinks they know the truth about %s, but recent revelations change everything.\n"+
		"Here is the breakdown of what actually happened behind closed doors.\n\n"+
		"TIMESTAMPS:\n%s\n"+
		"#%s #facts #viral #education #documentary",
		topic, chapters.String(), cleanTopic)

	tags := []string{
		strings.ToLower(topic),
		fmt.Sprintf("%s explained", strings.ToLower(topic)),
		fmt.Sprintf("%s truth", strings.ToLower(topic)),
		"untold story",
		"documentary",
		"deep dive",
		"investigation",
		"educational",
		"shorts",
		"viral video",
		"must watch",
		"history",
		"secrets",
		"analysis",
		"breakdown",
	}

	return &SEOMetadata{
		TitleCandidates: candidates,
		ChosenTitle:     chosenTitle,
		Description:     desc,
		Tags:            tags,
	}
}

// InitSeo initializes SEO package context (retained for backward compatibility).
func InitSeo(cfg *config.ConfigLoader, links *[]string) {
	ctx := context.Background()
	client, err := api.ClientFromEnvironment()
	if err != nil {
		if cfg != nil && cfg.Logger != nil {
			cfg.Logger.LogFileOnly("Ollama client unavailable for SEO", 1)
		}
		return
	}
	_ = ctx
	_ = client
	_ = links
}
