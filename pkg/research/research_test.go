package research

import (
	"testing"
	"time"
)

func TestMetricsAndCleanVTT(t *testing.T) {
	now := time.Now().UTC()
	pubAt := now.Add(-10 * time.Hour)

	vel := CalculateVelocity(1200, pubAt, now)
	// Hours = 10, so 1200 / (10 + 2) = 100
	if vel != 100 {
		t.Errorf("expected velocity 100, got %f", vel)
	}

	eng := CalculateEngagementRate(1000, 80, 20)
	// (80 + 20) / 1000 * 100 = 10%
	if eng != 10.0 {
		t.Errorf("expected engagement rate 10.0, got %f", eng)
	}

	items := []VideoResearchItem{
		{VideoID: "1", Velocity: 50.0},
		{VideoID: "2", Velocity: 150.0},
		{VideoID: "3", Velocity: 90.0},
	}
	ranked := RankVideosByVelocity(items)
	if ranked[0].VideoID != "2" || ranked[1].VideoID != "3" || ranked[2].VideoID != "1" {
		t.Errorf("unexpected ranking order: %+v", ranked)
	}

	rawVTT := `WEBVTT
Kind: captions
Language: en

1
00:00:01.000 --> 00:00:04.000
<c>Hello world!</c>

2
00:00:04.000 --> 00:00:06.000
Hello world!
This is Gozart.
`
	cleaned := CleanVTT(rawVTT)
	expected := "Hello world! This is Gozart."
	if cleaned != expected {
		t.Errorf("expected clean VTT %q, got %q", expected, cleaned)
	}
}
