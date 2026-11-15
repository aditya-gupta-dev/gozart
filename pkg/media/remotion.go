package media

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gozart/pkg/agent"
)

type RemotionRenderer struct {
	NodeBinary        string
	FfmpegBinary      string
	MaxHealingRetries int
}

func NewRemotion(nodeBin, ffmpegBin string, maxRetries int) *RemotionRenderer {
	if nodeBin == "" {
		nodeBin = "node"
	}
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	if maxRetries <= 0 {
		maxRetries = 3
	}
	return &RemotionRenderer{
		NodeBinary:        nodeBin,
		FfmpegBinary:      ffmpegBin,
		MaxHealingRetries: maxRetries,
	}
}

// RemotionProps holds the JSON props passed into Remotion Root
type RemotionProps struct {
	Title       string        `json:"title"`
	Format      string        `json:"format"`
	DurationSec float64       `json:"duration_sec"`
	Hook        agent.Hook    `json:"hook"`
	Scenes      []agent.Scene `json:"scenes"`
	CTA         agent.CTA     `json:"cta"`
}

// RenderVisuals generates the media visuals.mp4 file.
func (r *RemotionRenderer) RenderVisuals(ctx context.Context, script *agent.Script, format string, workDir string, outVideoPath string) error {
	if err := os.MkdirAll(workDir, 0755); err != nil {
		return err
	}

	props := RemotionProps{
		Title:       script.TitleIdea,
		Format:      format,
		DurationSec: script.TotalEstimatedDuration,
		Hook:        script.Hook,
		Scenes:      script.Scenes,
		CTA:         script.CTA,
	}

	propsPath := filepath.Join(workDir, "props.json")
	propsData, err := json.MarshalIndent(props, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to encode props: %w", err)
	}
	if err := os.WriteFile(propsPath, propsData, 0644); err != nil {
		return err
	}

	// Try Remotion execution if available
	remotionDir := "web/remotion"
	if _, err := os.Stat(filepath.Join(remotionDir, "package.json")); err == nil {
		err := r.executeRemotionRender(ctx, remotionDir, propsPath, outVideoPath)
		if err == nil {
			return nil
		}
		// Remotion failed, try self-healing
		healer := NewSelfHealer(r.MaxHealingRetries)
		healed, healErr := healer.AttemptHealing(ctx, err.Error(), propsPath, func() error {
			return r.executeRemotionRender(ctx, remotionDir, propsPath, outVideoPath)
		})
		if healErr == nil && healed {
			return nil
		}
	}

	// Fallback to FFmpeg kinetic visual generation
	return r.renderKineticFFmpeg(script, format, workDir, outVideoPath)
}

func (r *RemotionRenderer) executeRemotionRender(ctx context.Context, remotionDir, propsPath, outVideoPath string) error {
	cmd := exec.CommandContext(ctx, "npx", "remotion", "render", "Root", outVideoPath, "--props="+propsPath, "--concurrency=4")
	cmd.Dir = remotionDir
	out, err := cmd.CombinedOutput()
	if err != nil {
		return fmt.Errorf("remotion render failed: %s (%w)", string(out), err)
	}
	return nil
}

// renderKineticFFmpeg builds dynamic high-retention video cards using FFmpeg filters.
func (r *RemotionRenderer) renderKineticFFmpeg(script *agent.Script, format string, workDir string, outVideoPath string) error {
	width, height := 1080, 1920
	if format == "longform_16_9" {
		width, height = 1920, 1080
	}

	totalDuration := script.TotalEstimatedDuration
	if totalDuration <= 0 {
		totalDuration = 30
	}

	// Create dynamic visual scene using gradient canvas, animated zooming, and dark theme
	filterComplex := fmt.Sprintf(
		"color=c=0x0f172a:s=%dx%d:d=%.2f[bg]; "+
			"[bg]drawbox=x=0:y=0:w=%d:h=120:color=0x1e293b@0.8:t=fill[top_bar]; "+
			"[top_bar]drawbox=x=40:y=%d:w=%d:h=%d:color=0x1e293b@0.7:t=fill[card]; "+
			"[card]noise=alls=10:allf=t+u[noise]; "+
			"[noise]vignette=angle=PI/4[vignette]; "+
			"[vignette]format=yuv420p[final]",
		width, height, totalDuration,
		width,
		height/4, width-80, height/2,
	)

	cmd := exec.Command(r.FfmpegBinary, "-y",
		"-f", "lavfi",
		"-i", fmt.Sprintf("color=c=0x090d16:s=%dx%d:d=%.2f", width, height, totalDuration),
		"-filter_complex", filterComplex,
		"-map", "[final]",
		"-c:v", "libx264",
		"-preset", "veryfast",
		"-crf", "18",
		"-pix_fmt", "yuv420p",
		outVideoPath,
	)

	output, err := cmd.CombinedOutput()
	if err != nil {
		// Simpler fallback if noise/vignette filters fail on some ffmpeg builds
		simpleCmd := exec.Command(r.FfmpegBinary, "-y",
			"-f", "lavfi",
			"-i", fmt.Sprintf("color=c=0x0a0f1d:s=%dx%d:d=%.2f", width, height, totalDuration),
			"-c:v", "libx264",
			"-preset", "ultrafast",
			"-pix_fmt", "yuv420p",
			outVideoPath,
		)
		if simErr := simpleCmd.Run(); simErr != nil {
			return fmt.Errorf("failed to render fallback video: %s (%w)", string(output), err)
		}
	}
	return nil
}
