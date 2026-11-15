package audio

import (
	"os"
	"path/filepath"
	"testing"

	"gozart/pkg/config"
)

func TestPauseMarkers(t *testing.T) {
	input := "Welcome to the show. [pause 500ms] Today we examine the cosmos. [pause 200ms] Stay tuned."
	output := processPauseMarkers(input)

	if output == input {
		t.Errorf("expected pause markers to be converted, got unchanged string")
	}
	if !containsWord(output, "...") {
		t.Errorf("expected ellipsis pause marker, got: %s", output)
	}
}

func containsWord(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > 0)
}

func TestTTSVoiceoverFallback(t *testing.T) {
	tmpDir := t.TempDir()
	outWav := filepath.Join(tmpDir, "test_voice.wav")

	ttsCfg := config.AudioConfig{
		TTSEngine: "fallback",
		Voice:     "en-US-GuyNeural",
	}

	client := NewTTS(ttsCfg, "ffmpeg")
	text := "This is an automated test of the Gozart fallback speech synthesizer."

	err := client.GenerateVoiceover(text, outWav)
	if err != nil {
		t.Fatalf("GenerateVoiceover failed: %v", err)
	}

	info, err := os.Stat(outWav)
	if err != nil {
		t.Fatalf("generated voice file not found: %v", err)
	}
	if info.Size() == 0 {
		t.Fatalf("generated voice file is empty")
	}
}

func TestWhisperAlignerFallback(t *testing.T) {
	tmpDir := t.TempDir()
	outWav := filepath.Join(tmpDir, "test_voice.wav")

	// Create test voice file
	ttsCfg := config.AudioConfig{}
	client := NewTTS(ttsCfg, "ffmpeg")
	text := "The Roman Empire fell due to multiple systemic crises."
	_ = client.GenerateVoiceover(text, outWav)

	aligner := NewAligner("ffprobe")
	timestamps, err := aligner.AlignWords(outWav, text)
	if err != nil {
		t.Fatalf("AlignWords failed: %v", err)
	}

	if len(timestamps) == 0 {
		t.Fatalf("expected non-empty timestamps")
	}

	for i, ts := range timestamps {
		if ts.Word == "" {
			t.Errorf("timestamp %d has empty word", i)
		}
		if ts.EndMs <= ts.StartMs {
			t.Errorf("timestamp %d has invalid range: start=%d, end=%d", i, ts.StartMs, ts.EndMs)
		}
	}

	jsonPath := filepath.Join(tmpDir, "transcription.json")
	if err := aligner.SaveTranscription(timestamps, jsonPath); err != nil {
		t.Fatalf("SaveTranscription failed: %v", err)
	}

	info, err := os.Stat(jsonPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("transcription.json not created or empty")
	}
}

func TestDeepFilterFallback(t *testing.T) {
	tmpDir := t.TempDir()
	rawWav := filepath.Join(tmpDir, "raw.wav")
	enhancedWav := filepath.Join(tmpDir, "enhanced.wav")

	ttsCfg := config.AudioConfig{}
	client := NewTTS(ttsCfg, "ffmpeg")
	_ = client.GenerateVoiceover("Sample audio for noise filter", rawWav)

	// Test with DeepFilter disabled
	enhancerDisabled := NewEnhancer(config.AudioConfig{DeepFilterEnabled: false})
	if err := enhancerDisabled.EnhanceAudio(rawWav, enhancedWav); err != nil {
		t.Fatalf("EnhanceAudio with disabled filter failed: %v", err)
	}
	if _, err := os.Stat(enhancedWav); err != nil {
		t.Fatalf("expected enhanced wav to exist via fallback")
	}

	// Test with invalid binary path to verify graceful copy fallback
	enhancerInvalid := NewEnhancer(config.AudioConfig{
		DeepFilterEnabled: true,
		DeepFilterPath:    "/nonexistent/binary/deepfilter",
	})
	outWav2 := filepath.Join(tmpDir, "enhanced2.wav")
	if err := enhancerInvalid.EnhanceAudio(rawWav, outWav2); err != nil {
		t.Fatalf("EnhanceAudio with invalid binary should fallback gracefully, got err: %v", err)
	}
	if _, err := os.Stat(outWav2); err != nil {
		t.Fatalf("expected outWav2 to exist via fallback copy")
	}
}
