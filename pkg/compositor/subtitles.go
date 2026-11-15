package compositor

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"gozart/pkg/audio"
)

// GenerateASS creates animated, bold, karaoke/yellow-highlighted subtitles in ASS format.
func GenerateASS(words []audio.WordTimestamp, format string, outPath string) error {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	resX, resY := 1080, 1920
	fontSize := 72
	marginV := 900 // center screen for 9:16 Shorts
	alignment := 5 // center

	if format == "longform_16_9" {
		resX, resY = 1920, 1080
		fontSize = 54
		marginV = 140
		alignment = 2 // bottom center
	}

	var sb strings.Builder
	sb.WriteString("[Script Info]\n")
	sb.WriteString("Title: Gozart Dynamic Animated Captions\n")
	sb.WriteString("ScriptType: v4.00+\n")
	sb.WriteString("WrapStyle: 0\n")
	sb.WriteString(fmt.Sprintf("PlayResX: %d\n", resX))
	sb.WriteString(fmt.Sprintf("PlayResY: %d\n", resY))
	sb.WriteString("ScaledBorderAndShadow: yes\n\n")

	sb.WriteString("[V4+ Styles]\n")
	sb.WriteString("Format: Name, Fontname, Fontsize, PrimaryColour, SecondaryColour, OutlineColour, BackColour, Bold, Italic, Underline, StrikeOut, ScaleX, ScaleY, Spacing, Angle, BorderStyle, Outline, Shadow, Alignment, MarginL, MarginR, MarginV, Encoding\n")
	// Primary: White (&H00FFFFFF), Secondary/Karaoke: Yellow (&H0000FFFF), Outline: Black (&H00000000)
	sb.WriteString(fmt.Sprintf("Style: Default,Arial Black,%d,&H00FFFFFF,&H0000FFFF,&H00000000,&H80000000,-1,0,0,0,100,100,2,0,1,6,2,%d,60,60,%d,1\n",
		fontSize, alignment, marginV))
	sb.WriteString(fmt.Sprintf("Style: ActiveWord,Arial Black,%d,&H0000FFFF,&H0000FFFF,&H00000000,&H80000000,-1,0,0,0,110,110,2,0,1,8,3,%d,60,60,%d,1\n\n",
		fontSize+4, alignment, marginV))

	sb.WriteString("[Events]\n")
	sb.WriteString("Format: Layer, Start, End, Style, Name, MarginL, MarginR, MarginV, Effect, Text\n")

	if len(words) == 0 {
		return os.WriteFile(outPath, []byte(sb.String()), 0644)
	}

	// Chunk words into punchy phrases (3-4 words per subtitle screen)
	chunkSize := 3
	for i := 0; i < len(words); i += chunkSize {
		endIdx := i + chunkSize
		if endIdx > len(words) {
			endIdx = len(words)
		}
		chunk := words[i:endIdx]
		if len(chunk) == 0 {
			continue
		}

		// For each word in this chunk, create a dialogue line with the active word highlighted
		for currentWordIdx, active := range chunk {
			endMs := active.EndMs
			if endMs <= active.StartMs {
				endMs = active.StartMs + 200
			}
			start := formatASSTime(active.StartMs)
			end := formatASSTime(endMs)

			var lineBuilder strings.Builder
			for wIdx, w := range chunk {
				wordUpper := strings.ToUpper(w.Word)
				if wIdx == currentWordIdx {
					// Yellow bold active word with bounce scale
					lineBuilder.WriteString(fmt.Sprintf("{\\c&H00FFFF&\\fscx115\\fscy115}%s{\\r} ", wordUpper))
				} else {
					// White inactive word
					lineBuilder.WriteString(fmt.Sprintf("{\\c&HFFFFFF&}%s ", wordUpper))
				}
			}

			dialogueLine := fmt.Sprintf("Dialogue: 0,%s,%s,Default,,0,0,0,,%s\n",
				start, end, strings.TrimSpace(lineBuilder.String()))
			sb.WriteString(dialogueLine)
		}
	}

	return os.WriteFile(outPath, []byte(sb.String()), 0644)
}

func formatASSTime(ms int64) string {
	if ms < 0 {
		ms = 0
	}
	hours := ms / 3600000
	remainder := ms % 3600000
	minutes := remainder / 60000
	remainder = remainder % 60000
	seconds := remainder / 1000
	centiseconds := (remainder % 1000) / 10

	return fmt.Sprintf("%d:%02d:%02d.%02d", hours, minutes, seconds, centiseconds)
}
