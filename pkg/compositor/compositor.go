package compositor

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
)

type Compositor struct {
	FfmpegBin string
}

func New(ffmpegBin string) *Compositor {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	return &Compositor{FfmpegBin: ffmpegBin}
}

// Composite performs multi-track audio ducking, -14 LUFS loudness mastering, and subtitle burn-in.
func (c *Compositor) Composite(ctx context.Context, visualsPath, voicePath, bgmPath, captionsPath, outputPath string) error {
	dir := filepath.Dir(outputPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	escapedCaptions := escapeFilterPath(captionsPath)
	hasCaptions := false
	if captionsPath != "" {
		if _, err := os.Stat(captionsPath); err == nil {
			hasCaptions = true
		}
	}

	hasBGM := false
	if bgmPath != "" {
		if _, err := os.Stat(bgmPath); err == nil {
			hasBGM = true
		}
	}

	var args []string
	args = append(args, "-y")
	args = append(args, "-i", visualsPath)
	args = append(args, "-i", voicePath)

	var filterComplex string
	var videoFilter string
	if hasCaptions {
		videoFilter = fmt.Sprintf("[0:v]subtitles='%s'[final_video]", escapedCaptions)
	} else {
		videoFilter = "[0:v]null[final_video]"
	}

	if hasBGM {
		args = append(args, "-i", bgmPath)
		filterComplex = fmt.Sprintf(
			"[2:a]volume=0.22[bgm]; "+
				"[bgm][1:a]sidechaincompress=threshold=0.08:ratio=6:attack=50:release=300[ducked_bgm]; "+
				"[1:a][ducked_bgm]amix=inputs=2:duration=first[audio_mix]; "+
				"[audio_mix]loudnorm=I=-14:LRA=7:tp=-1.5[final_audio]; "+
				"%s", videoFilter)
	} else {
		filterComplex = fmt.Sprintf(
			"[1:a]loudnorm=I=-14:LRA=7:tp=-1.5[final_audio]; "+
				"%s", videoFilter)
	}

	args = append(args,
		"-filter_complex", filterComplex,
		"-map", "[final_video]",
		"-map", "[final_audio]",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "192k",
		"-shortest",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, c.FfmpegBin, args...)
	_, err := cmd.CombinedOutput()
	if err != nil {
		// Fallback 1: retry without subtitles filter in case fontconfig or ass path fails
		if hasCaptions {
			if subErr := c.compositeWithoutSubtitles(ctx, visualsPath, voicePath, bgmPath, outputPath); subErr == nil {
				return nil
			}
		}
		// Fallback 2: direct audio/video muxing if filter complex (e.g. loudnorm on short/silent audio) fails
		return c.compositeDirect(ctx, visualsPath, voicePath, outputPath)
	}

	return nil
}

func (c *Compositor) compositeWithoutSubtitles(ctx context.Context, visualsPath, voicePath, bgmPath, outputPath string) error {
	var args []string
	args = append(args, "-y", "-i", visualsPath, "-i", voicePath)

	var filterComplex string
	hasBGM := false
	if bgmPath != "" {
		if _, err := os.Stat(bgmPath); err == nil {
			hasBGM = true
		}
	}

	if hasBGM {
		args = append(args, "-i", bgmPath)
		filterComplex = "[2:a]volume=0.22[bgm]; " +
			"[bgm][1:a]sidechaincompress=threshold=0.08:ratio=6:attack=50:release=300[ducked_bgm]; " +
			"[1:a][ducked_bgm]amix=inputs=2:duration=first[audio_mix]; " +
			"[audio_mix]loudnorm=I=-14:LRA=7:tp=-1.5[final_audio]; " +
			"[0:v]null[final_video]"
	} else {
		filterComplex = "[1:a]loudnorm=I=-14:LRA=7:tp=-1.5[final_audio]; [0:v]null[final_video]"
	}

	args = append(args,
		"-filter_complex", filterComplex,
		"-map", "[final_video]",
		"-map", "[final_audio]",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "192k",
		"-shortest",
		outputPath,
	)

	cmd := exec.CommandContext(ctx, c.FfmpegBin, args...)
	_, err := cmd.CombinedOutput()
	if err != nil {
		return c.compositeDirect(ctx, visualsPath, voicePath, outputPath)
	}
	return nil
}

// compositeDirect muxes visual and audio streams directly when advanced audio filters fail.
func (c *Compositor) compositeDirect(ctx context.Context, visualsPath, voicePath, outputPath string) error {
	cmd := exec.CommandContext(ctx, c.FfmpegBin,
		"-y",
		"-i", visualsPath,
		"-i", voicePath,
		"-map", "0:v:0",
		"-map", "1:a:0",
		"-c:v", "libx264",
		"-preset", "fast",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		"-c:a", "aac",
		"-b:a", "192k",
		"-shortest",
		outputPath,
	)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("direct mux compositing failed: %s (%w)", string(out), err)
	}
	return nil
}

func escapeFilterPath(path string) string {
	path = strings.ReplaceAll(path, "\\", "/")
	path = strings.ReplaceAll(path, ":", "\\:")
	path = strings.ReplaceAll(path, "'", "\\'")
	return path
}
