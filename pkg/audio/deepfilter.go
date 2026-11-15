package audio

import (
	"fmt"
	"io"
	"os"
	"os/exec"
	"path/filepath"
	"strings"

	"gozart/pkg/config"
)

type DeepFilterEnhancer struct {
	Config config.AudioConfig
}

func NewEnhancer(cfg config.AudioConfig) *DeepFilterEnhancer {
	return &DeepFilterEnhancer{Config: cfg}
}

// EnhanceAudio applies DeepFilterNet v3 to remove noise and room reverb, producing enhanced_voice.wav.
func (e *DeepFilterEnhancer) EnhanceAudio(rawWavPath string, outWavPath string) error {
	dir := filepath.Dir(outWavPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	if !e.Config.DeepFilterEnabled {
		return copyFile(rawWavPath, outWavPath)
	}

	binPath := e.Config.DeepFilterPath
	if binPath == "" {
		binPath = "deep-filter"
	}

	if _, err := exec.LookPath(binPath); err != nil {
		// DeepFilter not in PATH, fallback to original WAV
		return copyFile(rawWavPath, outWavPath)
	}

	// deep-filter output directory
	tempOutDir := filepath.Join(dir, "df_temp")
	_ = os.MkdirAll(tempOutDir, 0755)
	defer os.RemoveAll(tempOutDir)

	cmd := exec.Command(binPath, rawWavPath, "-o", tempOutDir)
	if err := cmd.Run(); err != nil {
		// If deep-filter fails, gracefully fallback to raw audio
		return copyFile(rawWavPath, outWavPath)
	}

	// Locate output wav in tempOutDir
	entries, err := os.ReadDir(tempOutDir)
	if err != nil || len(entries) == 0 {
		return copyFile(rawWavPath, outWavPath)
	}

	var processedFile string
	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".wav") {
			processedFile = filepath.Join(tempOutDir, entry.Name())
			break
		}
	}

	if processedFile != "" {
		return copyFile(processedFile, outWavPath)
	}

	return copyFile(rawWavPath, outWavPath)
}

func copyFile(src, dst string) error {
	in, err := os.Open(src)
	if err != nil {
		return fmt.Errorf("failed to open source file: %w", err)
	}
	defer in.Close()

	out, err := os.Create(dst)
	if err != nil {
		return fmt.Errorf("failed to create dest file: %w", err)
	}
	defer out.Close()

	_, err = io.Copy(out, in)
	return err
}
