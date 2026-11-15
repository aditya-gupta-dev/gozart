package image

import (
	"image"
	"image/color"
	"image/jpeg"
	"math"
	"os"
	"os/exec"
	"path/filepath"
)

type ImageEnhancer struct {
	FfmpegBin string
}

func NewEnhancer(ffmpegBin string) *ImageEnhancer {
	if ffmpegBin == "" {
		ffmpegBin = "ffmpeg"
	}
	return &ImageEnhancer{FfmpegBin: ffmpegBin}
}

// EnhancePostProcessing increases saturation (+8%) and unsharp mask (+15%) for maximum mobile CTR.
func (e *ImageEnhancer) EnhancePostProcessing(inPath, outPath string) error {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	// Try using ffmpeg image filter if available
	cmd := exec.Command(e.FfmpegBin, "-y",
		"-i", inPath,
		"-vf", "eq=saturation=1.08:contrast=1.05,unsharp=5:5:0.8:3:3:0.4",
		outPath,
	)
	if err := cmd.Run(); err == nil {
		return nil
	}

	// Native Go fallback for saturation adjustment
	return e.enhanceNative(inPath, outPath)
}

func (e *ImageEnhancer) enhanceNative(inPath, outPath string) error {
	file, err := os.Open(inPath)
	if err != nil {
		return err
	}
	defer file.Close()

	src, err := jpeg.Decode(file)
	if err != nil {
		return err
	}

	bounds := src.Bounds()
	dst := image.NewRGBA(bounds)

	for y := bounds.Min.Y; y < bounds.Max.Y; y++ {
		for x := bounds.Min.X; x < bounds.Max.X; x++ {
			c := src.At(x, y)
			r, g, b, a := c.RGBA()
			rf := float64(r >> 8)
			gf := float64(g >> 8)
			bf := float64(b >> 8)

			// Simple HSV-like saturation boost (+8%)
			gray := 0.299*rf + 0.587*gf + 0.114*bf
			satFactor := 1.08
			rf = math.Max(0, math.Min(255, gray+(rf-gray)*satFactor))
			gf = math.Max(0, math.Min(255, gray+(gf-gray)*satFactor))
			bf = math.Max(0, math.Min(255, gray+(bf-gray)*satFactor))

			dst.Set(x, y, color.RGBA{
				R: uint8(rf),
				G: uint8(gf),
				B: uint8(bf),
				A: uint8(a >> 8),
			})
		}
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return jpeg.Encode(outFile, dst, &jpeg.Options{Quality: 95})
}
