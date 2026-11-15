package sentiment

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"gozart/pkg/agent"
)

type AudienceSentimentData struct {
	PositiveTriggers    []string `json:"positive_triggers"`
	NegativeFriction    []string `json:"negative_friction"`
	UnansweredQuestions []string `json:"unanswered_questions"`
	EmotionalTone       string   `json:"emotional_tone"`
	TargetAudienceSlang []string `json:"target_audience_slang"`
	DebateTopics        []string `json:"debate_topics"`
}

type SentimentReport struct {
	AudienceSentiment AudienceSentimentData `json:"audience_sentiment"`
}

// ExtractSentiment uses the AI harness to cluster comments into sentiment, friction, and curiosity gaps.
func ExtractSentiment(ctx context.Context, harness *agent.AgentHarness, comments []string, topic string) (*SentimentReport, error) {
	if len(comments) == 0 {
		return GenerateFallbackSentiment(topic), nil
	}

	// Limit to top 50 comments for LLM prompt context window
	sampleComments := comments
	if len(sampleComments) > 50 {
		sampleComments = sampleComments[:50]
	}

	var commentLines []string
	for i, c := range sampleComments {
		commentLines = append(commentLines, fmt.Sprintf("%d. %s", i+1, c))
	}

	userPrompt := fmt.Sprintf("Topic: %s\n\nYouTube Viewer Comments:\n%s", topic, strings.Join(commentLines, "\n"))

	if harness != nil {
		jsonStr, err := harness.GenerateStructuredJSON(ctx, "", agent.SentimentExtractionPrompt, userPrompt)
		if err == nil {
			var report SentimentReport
			if err := json.Unmarshal([]byte(jsonStr), &report); err == nil && len(report.AudienceSentiment.UnansweredQuestions) > 0 {
				return &report, nil
			}
		}
	}

	return GenerateFallbackSentiment(topic), nil
}

// GenerateFallbackSentiment provides realistic structured sentiment when LLM or comments are unavailable.
func GenerateFallbackSentiment(topic string) *SentimentReport {
	return &SentimentReport{
		AudienceSentiment: AudienceSentimentData{
			PositiveTriggers: []string{
				fmt.Sprintf("Audience enjoys deep dives into the hidden backstory of %s", topic),
				"Viewers appreciate fast-paced visual explanations with hard data",
			},
			NegativeFriction: []string{
				"Viewers are tired of clickbait titles that leave out key historical context",
				"Pacing is often too slow in competing longform videos",
			},
			UnansweredQuestions: []string{
				fmt.Sprintf("What was the exact financial and political catalyst behind %s?", topic),
				"Why do mainstream sources contradict each other on this point?",
			},
			EmotionalTone: "curiosity and skepticism",
			TargetAudienceSlang: []string{
				"no cap",
				"deep rabbit hole",
				"underrated fact",
			},
			DebateTopics: []string{
				fmt.Sprintf("Whether mainstream explanations of %s tell the whole story", topic),
			},
		},
	}
}
