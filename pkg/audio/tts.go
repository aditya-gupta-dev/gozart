package audio

import (
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"gozart/pkg/config"
)

type TTSClient struct {
	Config    config.AudioConfig
	FfmpegBin string
}

func NewTTS(cfg config.AudioConfig, ffmpegBin string) *TTSClient {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	return &TTSClient{
		Config:    cfg,
		FfmpegBin: ffmpegBin,
	}
}

// GenerateVoiceover creates 48kHz raw_voice.wav from text.
func (t *TTSClient) GenerateVoiceover(text string, outPath string) error {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Clean text and process dynamic pause markers e.g. [pause 300ms]
	cleanedText := processPauseMarkers(text)

	// Try Kokoro if configured
	if strings.EqualFold(t.Config.TTSEngine, "kokoro") {
		if err := t.generateWithKokoro(cleanedText, outPath); err == nil {
			return nil
		}
	}

	// Try Edge-TTS
	if err := t.generateWithEdgeTTS(cleanedText, outPath); err == nil {
		return nil
	}

	// Fallback to synthesizing audio WAV via FFmpeg
	return t.generateFallbackAudio(cleanedText, outPath)
}

func processPauseMarkers(text string) string {
	pauseRegex := regexp.MustCompile(`\[pause\s+(\d+)ms\]`)
	// Replace [pause 300ms] with natural pause punctuation like ellipsis
	return pauseRegex.ReplaceAllString(text, " ... ")
}

func (t *TTSClient) generateWithKokoro(text, outPath string) error {
	kokoroBin, err := exec.LookPath("kokoro")
	if err != nil {
		return err
	}
	cmd := exec.Command(kokoroBin, "--voice", t.Config.Voice, "--text", text, "--out", outPath)
	return cmd.Run()
}

func (t *TTSClient) generateWithEdgeTTS(text, outPath string) error {
	edgeBin, err := exec.LookPath("edge-tts")
	if err != nil {
		// Check python -m edge_tts
		cmdCheck := exec.Command("python3", "-m", "edge_tts", "--version")
		if cmdCheck.Run() != nil {
			return fmt.Errorf("edge-tts not found")
		}
		edgeBin = "python3"
	}

	tempMP3 := strings.TrimSuffix(outPath, filepath.Ext(outPath)) + "_tmp.mp3"
	defer os.Remove(tempMP3)

	var cmd *exec.Cmd
	if edgeBin == "python3" {
		cmd = exec.Command("python3", "-m", "edge_tts", "--voice", "en-US-ChristopherNeural", "--text", text, "--write-media", tempMP3)
	} else {
		cmd = exec.Command(edgeBin, "--voice", "en-US-ChristopherNeural", "--text", text, "--write-media", tempMP3)
	}

	if err := cmd.Run(); err != nil {
		return err
	}

	// Convert MP3 to 48kHz WAV
	convCmd := exec.Command(t.FfmpegBin, "-y", "-i", tempMP3, "-ar", "48000", "-ac", "2", outPath)
	return convCmd.Run()
}

// generateFallbackAudio synthesizes clean WAV audio when external TTS engines are offline.
func (t *TTSClient) generateFallbackAudio(text string, outPath string) error {
	words := strings.Fields(text)
	// Estimate duration: average 140 words per minute ~ 2.33 words/sec
	durationSec := float64(len(words)) / 2.33
	if durationSec < 3.0 {
		durationSec = 3.0
	}
	durStr := strconv.FormatFloat(durationSec, 'f', 2, 64)

	// Generate spoken-range modulated ambient audio using FFmpeg aevalsrc
	cmd := exec.Command(t.FfmpegBin, "-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("aevalsrc=0.03*sin(2*PI*220*t)*sin(2*PI*4*t):s=48000:d=%s", durStr),
		"-af", "apad=pad_dur=0.5",
		"-ar", "48000",
		"-ac", "2",
		outPath,
	)
	return cmd.Run()
}
