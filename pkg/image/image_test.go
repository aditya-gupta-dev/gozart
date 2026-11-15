package image

import (
	"context"
	"image"
	"image/color"
	"image/jpeg"
	"os"
	"path/filepath"
	"testing"
)

func TestCraftPromptFallback(t *testing.T) {
	gen := NewGenerator("http://localhost:9999")
	spec, err := gen.CraftPrompt(context.Background(), nil, "Ancient Rome", "Did Rome really fall in a day?")
	if err != nil {
		t.Fatalf("CraftPrompt returned unexpected error: %v", err)
	}
	if spec == nil {
		t.Fatalf("expected non-nil spec")
	}
	if spec.ImagePrompt == "" {
		t.Errorf("expected non-empty ImagePrompt")
	}
	if spec.OverlayText == "" {
		t.Errorf("expected non-empty OverlayText")
	}
}

func TestGenerateNativeThumbnail(t *testing.T) {
	tmpDir := t.TempDir()
	outPath := filepath.Join(tmpDir, "thumb.jpg")

	gen := NewGenerator("http://invalid-comfy-server:9999")
	spec := &ThumbnailSpec{
		ImagePrompt: "Test prompt",
		OverlayText: "EXPOSED",
		ColorScheme: "Neon Yellow and Navy",
	}

	err := gen.Generate(context.Background(), spec, "shorts_9_16", outPath)
	if err != nil {
		t.Fatalf("Generate failed: %v", err)
	}

	info, err := os.Stat(outPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("thumbnail file was not created or empty")
	}

	f, err := os.Open(outPath)
	if err != nil {
		t.Fatalf("failed to open thumbnail: %v", err)
	}
	defer f.Close()

	img, err := jpeg.Decode(f)
	if err != nil {
		t.Fatalf("failed to decode thumbnail jpeg: %v", err)
	}

	bounds := img.Bounds()
	if bounds.Dx() != 1080 || bounds.Dy() != 1920 {
		t.Errorf("expected 1080x1920 bounds for shorts, got %dx%d", bounds.Dx(), bounds.Dy())
	}
}

func TestImageEnhancer(t *testing.T) {
	tmpDir := t.TempDir()
	rawPath := filepath.Join(tmpDir, "raw.jpg")
	enhancedPath := filepath.Join(tmpDir, "enhanced.jpg")

	// Create a synthetic test JPEG
	img := image.NewRGBA(image.Rect(0, 0, 100, 100))
	for y := 0; y < 100; y++ {
		for x := 0; x < 100; x++ {
			img.Set(x, y, color.RGBA{R: uint8(x * 2), G: uint8(y * 2), B: 128, A: 255})
		}
	}
	f, _ := os.Create(rawPath)
	_ = jpeg.Encode(f, img, &jpeg.Options{Quality: 90})
	_ = f.Close()

	enhancer := NewEnhancer("ffmpeg")
	if err := enhancer.EnhancePostProcessing(rawPath, enhancedPath); err != nil {
		t.Fatalf("EnhancePostProcessing failed: %v", err)
	}

	info, err := os.Stat(enhancedPath)
	if err != nil || info.Size() == 0 {
		t.Fatalf("enhanced image file missing or empty")
	}

	// Also verify native Go enhancement fallback
	nativeEnhancedPath := filepath.Join(tmpDir, "native_enhanced.jpg")
	enhancerInvalid := NewEnhancer("/nonexistent/ffmpeg")
	if err := enhancerInvalid.EnhancePostProcessing(rawPath, nativeEnhancedPath); err != nil {
		t.Fatalf("native Go enhancer fallback failed: %v", err)
	}
	if info2, err := os.Stat(nativeEnhancedPath); err != nil || info2.Size() == 0 {
		t.Fatalf("native enhanced image missing or empty")
	}
}
