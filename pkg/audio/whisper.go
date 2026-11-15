package audio

import (
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"
	"unicode/utf8"
)

type WordTimestamp struct {
	Word    string `json:"word"`
	StartMs int64  `json:"start_ms"`
	EndMs   int64  `json:"end_ms"`
}

type WhisperAligner struct {
	FfprobeBin string
}

func NewAligner(ffprobeBin string) *WhisperAligner {
	if ffprobeBin == "" {
		ffprobeBin = "ffprobe"
	}
	return &WhisperAligner{FfprobeBin: ffprobeBin}
}

// AlignWords calculates word-level millisecond timestamps mapped to the voiceover audio duration.
func (a *WhisperAligner) AlignWords(audioPath string, text string) ([]WordTimestamp, error) {
	if ts, err := a.alignWithWhisperCLI(audioPath); err == nil && len(ts) > 0 {
		return ts, nil
	}

	durationMs, err := a.getAudioDurationMs(audioPath)
	if err != nil {
		durationMs = 30000 // default fallback 30s
	}

	words := strings.Fields(text)
	if len(words) == 0 {
		return nil, nil
	}

	// Calculate word weights based on character length and trailing punctuation pauses
	type wordWeight struct {
		word   string
		weight float64
	}
	var weights []wordWeight
	var totalWeight float64

	for _, w := range words {
		cleaned := strings.Trim(w, " \t\r\n")
		charLen := float64(utf8.RuneCountInString(cleaned))
		if charLen < 1 {
			charLen = 1
		}
		weight := charLen

		// Add extra pause duration for punctuation
		if strings.HasSuffix(cleaned, ".") || strings.HasSuffix(cleaned, "!") || strings.HasSuffix(cleaned, "?") {
			weight += 4.0
		} else if strings.HasSuffix(cleaned, ",") || strings.HasSuffix(cleaned, ";") || strings.HasSuffix(cleaned, ":") {
			weight += 2.0
		}

		weights = append(weights, wordWeight{word: cleaned, weight: weight})
		totalWeight += weight
	}

	msPerWeight := float64(durationMs) / totalWeight
	var currentMs float64 = 0.0
	var timestamps []WordTimestamp

	for _, ww := range weights {
		wordDur := ww.weight * msPerWeight
		start := int64(currentMs)
		end := int64(currentMs + wordDur)
		if end > durationMs {
			end = durationMs
		}

		timestamps = append(timestamps, WordTimestamp{
			Word:    ww.word,
			StartMs: start,
			EndMs:   end,
		})
		currentMs += wordDur
	}

	return timestamps, nil
}

// SaveTranscription saves the word timestamps to transcription.json.
func (a *WhisperAligner) SaveTranscription(timestamps []WordTimestamp, outPath string) error {
	dir := filepath.Dir(outPath)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return err
	}

	data, err := json.MarshalIndent(timestamps, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(outPath, data, 0644)
}

func (a *WhisperAligner) getAudioDurationMs(audioPath string) (int64, error) {
	cmd := exec.Command(a.FfprobeBin,
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "default=noprint_wrappers=1:nokey=1",
		audioPath,
	)
	out, err := cmd.Output()
	if err != nil {
		return 0, fmt.Errorf("ffprobe failed: %w", err)
	}

	durSec, err := strconv.ParseFloat(strings.TrimSpace(string(out)), 64)
	if err != nil {
		return 0, fmt.Errorf("failed to parse audio duration: %w", err)
	}

	return int64(durSec * 1000.0), nil
}

func (a *WhisperAligner) alignWithWhisperCLI(audioPath string) ([]WordTimestamp, error) {
	cliName := "whisperx"
	if _, err := exec.LookPath(cliName); err != nil {
		cliName = "whisper"
		if _, err := exec.LookPath(cliName); err != nil {
			return nil, fmt.Errorf("no whisper binary in PATH")
		}
	}

	tempDir, err := os.MkdirTemp("", "whisper_align_*")
	if err != nil {
		return nil, err
	}
	defer os.RemoveAll(tempDir)

	var cmd *exec.Cmd
	if cliName == "whisperx" {
		cmd = exec.Command("whisperx", audioPath, "--output_dir", tempDir, "--output_format", "json")
	} else {
		cmd = exec.Command("whisper", audioPath, "--output_dir", tempDir, "--output_format", "json", "--word_timestamps", "True")
	}

	if err := cmd.Run(); err != nil {
		return nil, err
	}

	// Locate JSON output
	entries, err := os.ReadDir(tempDir)
	if err != nil {
		return nil, err
	}

	for _, entry := range entries {
		if strings.HasSuffix(entry.Name(), ".json") {
			data, err := os.ReadFile(filepath.Join(tempDir, entry.Name()))
			if err != nil {
				continue
			}

			type WhisperWord struct {
				Word  string  `json:"word"`
				Start float64 `json:"start"`
				End   float64 `json:"end"`
			}
			type WhisperSegment struct {
				Words []WhisperWord `json:"words"`
			}
			type WhisperOutput struct {
				Segments []WhisperSegment `json:"segments"`
				WordSegs []WhisperWord    `json:"word_segments"`
			}

			var out WhisperOutput
			if err := json.Unmarshal(data, &out); err == nil {
				var result []WordTimestamp
				allWords := out.WordSegs
				if len(allWords) == 0 {
					for _, seg := range out.Segments {
						allWords = append(allWords, seg.Words...)
					}
				}
				for _, w := range allWords {
					if w.Word != "" {
						result = append(result, WordTimestamp{
							Word:    strings.TrimSpace(w.Word),
							StartMs: int64(w.Start * 1000),
							EndMs:   int64(w.End * 1000),
						})
					}
				}
				if len(result) > 0 {
					return result, nil
				}
			}
		}
	}

	return nil, fmt.Errorf("no word timestamps found in whisper output")
}
