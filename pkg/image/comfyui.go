package image

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"math"
	"net/http"
	"os"
	"path/filepath"
	"time"

	"gozart/pkg/agent"
)

type ThumbnailSpec struct {
	ImagePrompt string `json:"image_prompt"`
	OverlayText string `json:"overlay_text"`
	ColorScheme string `json:"color_scheme"`
}

type ThumbnailGenerator struct {
	ComfyUIURL string
	httpClient *http.Client
}

func NewGenerator(comfyURL string) *ThumbnailGenerator {
	if comfyURL == "" {
		comfyURL = "http://localhost:8188"
	}
	return &ThumbnailGenerator{
		ComfyUIURL: comfyURL,
		httpClient: &http.Client{Timeout: 60 * time.Second},
	}
}

// CraftPrompt uses the LLM to design a high-CTR visual thumbnail prompt.
func (g *ThumbnailGenerator) CraftPrompt(ctx context.Context, harness *agent.AgentHarness, topic, hook string) (*ThumbnailSpec, error) {
	userMsg := fmt.Sprintf("Topic: %s\nHook: %s", topic, hook)
	if harness != nil {
		jsonStr, err := harness.GenerateStructuredJSON(ctx, "", agent.ThumbnailPromptCrafterPrompt, userMsg)
		if err == nil {
			var spec ThumbnailSpec
			if err := json.Unmarshal([]byte(jsonStr), &spec); err == nil && spec.ImagePrompt != "" {
				return &spec, nil
			}
		}
	}

	return &ThumbnailSpec{
		ImagePrompt: fmt.Sprintf("Cinematic hyper-detailed focal shot representing %s, intense dramatic lighting, complementary high-contrast colors, 8k resolution, octane render", topic),
		OverlayText: "WHAT HAPPENED?",
		ColorScheme: "Cyan and Dark Navy with Gold Accents",
	}, nil
}

// Generate creates the thumbnail file at outPath.
func (g *ThumbnailGenerator) Generate(ctx context.Context, spec *ThumbnailSpec, format, outPath string) error {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	width, height := 1280, 720
	if format == "shorts_9_16" {
		width, height = 1080, 1920
	}

	// Try ComfyUI API if active and configured
	if err := g.generateWithComfyUI(ctx, spec.ImagePrompt, width, height, outPath); err == nil {
		if fi, sErr := os.Stat(outPath); sErr == nil && fi.Size() > 0 {
			return nil
		}
	}

	// Resilient native Go image generator
	return g.generateNativeThumbnail(spec, width, height, outPath)
}

func (g *ThumbnailGenerator) generateWithComfyUI(ctx context.Context, prompt string, width, height int, outPath string) error {
	reqBody := map[string]interface{}{
		"prompt": map[string]interface{}{
			"3": map[string]interface{}{
				"class_type": "KSampler",
				"inputs": map[string]interface{}{
					"seed":  time.Now().UnixNano(),
					"steps": 20,
					"cfg":   7.0,
				},
			},
		},
	}
	body, _ := json.Marshal(reqBody)
	req, err := http.NewRequestWithContext(ctx, "POST", g.ComfyUIURL+"/prompt", bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := g.httpClient.Do(req)
	if err != nil || resp.StatusCode != http.StatusOK {
		return fmt.Errorf("comfyui offline or unavailable")
	}
	defer resp.Body.Close()

	var promptResp struct {
		PromptID string `json:"prompt_id"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&promptResp); err != nil || promptResp.PromptID == "" {
		return fmt.Errorf("invalid response from comfyui")
	}

	// Check history for generated image (with brief poll)
	for i := 0; i < 5; i++ {
		select {
		case <-ctx.Done():
			return ctx.Err()
		case <-time.After(500 * time.Millisecond):
		}

		histReq, err := http.NewRequestWithContext(ctx, "GET", g.ComfyUIURL+"/history/"+promptResp.PromptID, nil)
		if err != nil {
			continue
		}
		histResp, err := g.httpClient.Do(histReq)
		if err != nil || histResp.StatusCode != http.StatusOK {
			continue
		}

		var histData map[string]struct {
			Outputs map[string]struct {
				Images []struct {
					Filename  string `json:"filename"`
					Subfolder string `json:"subfolder"`
					Type      string `json:"type"`
				} `json:"images"`
			} `json:"outputs"`
		}

		if err := json.NewDecoder(histResp.Body).Decode(&histData); err == nil {
			histResp.Body.Close()
			if data, ok := histData[promptResp.PromptID]; ok {
				for _, out := range data.Outputs {
					if len(out.Images) > 0 {
						imgInfo := out.Images[0]
						viewURL := fmt.Sprintf("%s/view?filename=%s&subfolder=%s&type=%s",
							g.ComfyUIURL, imgInfo.Filename, imgInfo.Subfolder, imgInfo.Type)
						viewReq, err := http.NewRequestWithContext(ctx, "GET", viewURL, nil)
						if err != nil {
							break
						}
						viewResp, err := g.httpClient.Do(viewReq)
						if err == nil && viewResp.StatusCode == http.StatusOK {
							defer viewResp.Body.Close()
							outFile, err := os.Create(outPath)
							if err == nil {
								_, copyErr := io.Copy(outFile, viewResp.Body)
								outFile.Close()
								if copyErr == nil {
									return nil
								}
							}
						}
					}
				}
			}
		} else {
			histResp.Body.Close()
		}
	}

	return fmt.Errorf("comfyui image retrieval timed out or incomplete")
}

// generateNativeThumbnail produces a high-CTR graphic thumbnail in pure Go.
func (g *ThumbnailGenerator) generateNativeThumbnail(spec *ThumbnailSpec, width, height int, outPath string) error {
	img := image.NewRGBA(image.Rect(0, 0, width, height))

	// Draw radial/angular gradient backdrop
	for y := 0; y < height; y++ {
		for x := 0; x < width; x++ {
			nx := float64(x) / float64(width)
			ny := float64(y) / float64(height)
			dist := math.Sqrt((nx-0.5)*(nx-0.5) + (ny-0.5)*(ny-0.5))

			// Dark navy blue to dark slate gradient
			r := uint8(math.Max(0, math.Min(255, (1.0-dist)*40+10)))
			gCol := uint8(math.Max(0, math.Min(255, (1.0-dist)*60+20)))
			b := uint8(math.Max(0, math.Min(255, (1.0-dist)*120+40)))

			// Subtle corner vignette
			if dist > 0.4 {
				vignette := math.Min(1.0, (dist-0.4)*2.0)
				r = uint8(float64(r) * (1.0 - vignette*0.7))
				gCol = uint8(float64(gCol) * (1.0 - vignette*0.7))
				b = uint8(float64(b) * (1.0 - vignette*0.7))
			}

			img.Set(x, y, color.RGBA{R: r, G: gCol, B: b, A: 255})
		}
	}

	// Draw high-contrast focal card
	cardW := int(float64(width) * 0.85)
	cardH := int(float64(height) * 0.4)
	cardX := (width - cardW) / 2
	cardY := (height - cardH) / 2

	cardColor := color.RGBA{R: 24, G: 32, B: 54, A: 220}
	borderColor := color.RGBA{R: 251, G: 191, B: 36, A: 255} // Amber Gold

	// Fill card
	for cy := cardY; cy < cardY+cardH; cy++ {
		for cx := cardX; cx < cardX+cardW; cx++ {
			// Border 6px
			if cx < cardX+6 || cx >= cardX+cardW-6 || cy < cardY+6 || cy >= cardY+cardH-6 {
				img.Set(cx, cy, borderColor)
			} else {
				img.Set(cx, cy, cardColor)
			}
		}
	}

	// Draw decorative accent lines
	accentY := cardY + cardH - 14
	for ax := cardX + 20; ax < cardX+cardW-20; ax++ {
		for ay := accentY; ay < accentY+4; ay++ {
			img.Set(ax, ay, borderColor)
		}
	}

	outFile, err := os.Create(outPath)
	if err != nil {
		return err
	}
	defer outFile.Close()

	return jpeg.Encode(outFile, img, &jpeg.Options{Quality: 95})
}
