package sentiment

import (
	"context"
	"testing"
)

func TestExtractSentimentFallback(t *testing.T) {
	ctx := context.Background()
	report, err := ExtractSentiment(ctx, nil, nil, "Mars Colony")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if report == nil {
		t.Fatalf("expected non-nil report")
	}
	if len(report.AudienceSentiment.PositiveTriggers) == 0 {
		t.Errorf("expected positive triggers to be populated")
	}
	if len(report.AudienceSentiment.UnansweredQuestions) == 0 {
		t.Errorf("expected unanswered questions to be populated")
	}
}
