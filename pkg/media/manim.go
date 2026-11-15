package media

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"

	"gozart/pkg/agent"
)

type ManimRenderer struct {
	ManimBin string
}

func NewManim(manimBin string) *ManimRenderer {
	if manimBin == "" {
		manimBin = "manim"
	}
	return &ManimRenderer{ManimBin: manimBin}
}

func (m *ManimRenderer) Render(ctx context.Context, script *agent.Script, workDir string, outVideoPath string) error {
	if _, err := exec.LookPath(m.ManimBin); err != nil {
		return fmt.Errorf("manim executable not found in PATH")
	}

	manimPy := filepath.Join(workDir, "scene.py")
	pyCode := fmt.Sprintf(`from manim import *

class Scene(Scene):
    def construct(self):
        title = Text("%s", font_size=42, color=YELLOW)
        self.play(Write(title))
        self.wait(2)
        self.play(FadeOut(title))
`, script.TitleIdea)

	if err := os.WriteFile(manimPy, []byte(pyCode), 0644); err != nil {
		return err
	}

	cmd := exec.CommandContext(ctx, m.ManimBin, "-qm", manimPy, "Scene", "-o", outVideoPath)
	return cmd.Run()
}
