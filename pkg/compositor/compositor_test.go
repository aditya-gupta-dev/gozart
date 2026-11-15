package compositor

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"gozart/pkg/audio"
)

func TestSubtitlesASS(t *testing.T) {
	tempDir := t.TempDir()
	assPath := filepath.Join(tempDir, "captions.ass")

	words := []audio.WordTimestamp{
		{Word: "Hello", StartMs: 0, EndMs: 500},
		{Word: "Gozart", StartMs: 500, EndMs: 1200},
		{Word: "World", StartMs: 1200, EndMs: 1800},
	}

	err := GenerateASS(words, "shorts_9_16", assPath)
	if err != nil {
		t.Fatalf("failed to generate ASS: %v", err)
	}

	content, err := os.ReadFile(assPath)
	if err != nil {
		t.Fatalf("failed to read generated ASS: %v", err)
	}

	strContent := string(content)
	if !strings.Contains(strContent, "[Script Info]") {
		t.Errorf("missing [Script Info]")
	}
	if !strings.Contains(strContent, "Dialogue:") {
		t.Errorf("missing Dialogue lines")
	}
	if !strings.Contains(strContent, "0:00:00.00") {
		t.Errorf("missing start timestamp")
	}

	formatted := formatASSTime(65120) // 1 minute, 5 seconds, 12 centiseconds
	if formatted != "0:01:05.12" {
		t.Errorf("expected 0:01:05.12, got %s", formatted)
	}
}

func TestCompositeEndToEnd(t *testing.T) {
	tempDir := t.TempDir()
	visualsPath := filepath.Join(tempDir, "visuals.mp4")
	voicePath := filepath.Join(tempDir, "voice.wav")
	bgmPath := filepath.Join(tempDir, "bgm.mp3")
	captionsPath := filepath.Join(tempDir, "captions.ass")
	outputPath := filepath.Join(tempDir, "final.mp4")

	comp := New("ffmpeg")

	// Generate small 1-second synthetic video, voice, and BGM for the test
	cmdVisuals := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "color=c=black:s=640x360:d=1", "-c:v", "libx264", "-preset", "ultrafast", visualsPath)
	if err := cmdVisuals.Run(); err != nil {
		t.Skipf("ffmpeg not working or missing x264: %v", err)
	}

	cmdVoice := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=440:duration=1", "-ar", "48000", "-ac", "2", voicePath)
	if err := cmdVoice.Run(); err != nil {
		t.Fatalf("failed to create test voice: %v", err)
	}

	cmdBGM := exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "sine=frequency=220:duration=1", "-c:a", "mp3", bgmPath)
	if err := cmdBGM.Run(); err != nil {
		t.Fatalf("failed to create test BGM: %v", err)
	}

	words := []audio.WordTimestamp{
		{Word: "Test", StartMs: 0, EndMs: 400},
		{Word: "Video", StartMs: 400, EndMs: 900},
	}
	if err := GenerateASS(words, "shorts_9_16", captionsPath); err != nil {
		t.Fatalf("failed to generate test ASS: %v", err)
	}

	ctx := context.Background()
	err := comp.Composite(ctx, visualsPath, voicePath, bgmPath, captionsPath, outputPath)
	if err != nil {
		t.Fatalf("composite failed: %v", err)
	}

	fi, err := os.Stat(outputPath)
	if err != nil {
		t.Fatalf("output video does not exist: %v", err)
	}
	if fi.Size() == 0 {
		t.Errorf("output video is empty")
	}
}
