package editor

import (
	"encoding/json"
	"fmt"
	"math"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"gozart/pkg/config"
	"gozart/pkg/logger"
)

type VideoEditor struct {
	Link                     string
	Logger                   *logger.Logger
	ConfigLoader             *config.ConfigLoader
	Failed                   bool
	AlreadyAsMp4             bool
	VideoID                  string
	FinalOutputVideoDuration int
}

func New(link string, l *logger.Logger, cfg *config.ConfigLoader, videoID string) *VideoEditor {
	l.LogFileWithStdout("Started Editing [ "+videoID+" ]", logger.Info)
	return &VideoEditor{
		Link:                     link,
		Logger:                   l,
		ConfigLoader:             cfg,
		VideoID:                  videoID,
		FinalOutputVideoDuration: 30,
	}
}

func (e *VideoEditor) GetVideoTitle() string {
	apiKey := e.ConfigLoader.ConfigData.YoutubeAPIKey
	apiURL := fmt.Sprintf("https://www.googleapis.com/youtube/v3/videos?id=%s&part=snippet&key=%s", e.VideoID, apiKey)

	resp, err := http.Get(apiURL)
	if err != nil || resp.StatusCode != http.StatusOK {
		e.Logger.LogFileWithStdout("Failed to request youtube or status not OK", logger.Error)
		return "Viral Song"
	}
	defer resp.Body.Close()

	var result struct {
		Items []struct {
			Snippet struct {
				Title string `json:"title"`
			} `json:"snippet"`
		} `json:"items"`
	}

	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil || len(result.Items) == 0 {
		return "Viral Song"
	}

	return result.Items[0].Snippet.Title
}

func (e *VideoEditor) GenerateOutputFilename(ytTitle string) string {
	re := regexp.MustCompile(`[^a-zA-Z0-9_]`)
	sanitizedTitle := re.ReplaceAllString(ytTitle, " ")
	
	duration := e.ConfigLoader.GetFinalVideoDuration()
	hours := math.Round(float64(duration) / 3600.0)
	suffix := fmt.Sprintf(" %.0f Hour looped", hours)

	limit := 100
	requiredLength := limit - len(suffix)

	if len(sanitizedTitle) > requiredLength {
		return sanitizedTitle[:requiredLength] + suffix
	}
	return sanitizedTitle + suffix
}

func (e *VideoEditor) Edit() {
	e.ConvertToMp4()
	e.ExtractAudioFromVideo()
	e.MergeAssetAndAudioFile()
	e.GetVideoDuration()
	e.GenerateConcatDemuxerFile()
	e.RenderFinalOutputVideo()
	e.DownloadOriginalThumbnail()
}

func (e *VideoEditor) runFfmpeg(args []string, step string) {
	if e.Failed {
		return
	}
	
	cmd := exec.Command(e.ConfigLoader.ConfigData.FfmpegPath, args...)
	if err := cmd.Run(); err != nil {
		e.Logger.LogFileWithStdout(fmt.Sprintf("Failed %s", step), logger.Error)
		e.Failed = true
	} else {
		e.Logger.LogFileWithStdout(fmt.Sprintf("Completed %s", step), logger.Info)
	}
}

func (e *VideoEditor) ConvertToMp4() {
	savedDir := filepath.Join("files", e.VideoID)
	inputWebm := filepath.Join(savedDir, "input.webm")
	inputMp4 := filepath.Join(savedDir, "input.mp4")
	outputMp4 := filepath.Join(savedDir, "output.mp4")

	if _, err := os.Stat(inputMp4); err == nil {
		e.AlreadyAsMp4 = true
		e.Logger.LogFileWithStdout("Video already downloaded as .mp4, skipping conversion", logger.Info)
		return
	}

	if _, err := os.Stat(outputMp4); err == nil {
		e.Logger.LogFileWithStdout("output.mp4 already exists. skipping this step!!", logger.Info)
		return
	}

	e.Logger.LogFileWithStdout(fmt.Sprintf("Converting [ %s ] to mp4", inputWebm), logger.Info)
	e.runFfmpeg([]string{"-i", inputWebm, "-c", "copy", outputMp4}, "converting to mp4")
}

func (e *VideoEditor) ExtractAudioFromVideo() {
	savedDir := filepath.Join("files", e.VideoID)
	filename := "input.webm"
	if e.AlreadyAsMp4 {
		filename = "input.mp4"
	}
	
	inputFile := filepath.Join(savedDir, filename)
	audioFile := filepath.Join(savedDir, "audio.mp3")

	if _, err := os.Stat(audioFile); err == nil {
		e.Logger.LogFileWithStdout("audio.mp3 already exists. Skipping this step !!", logger.Info)
		return
	}

	e.Logger.LogFileWithStdout(fmt.Sprintf("Extracting audio from [ %s ]", inputFile), logger.Info)
	e.runFfmpeg([]string{"-i", inputFile, "-q:a", "0", "-map", "a", audioFile}, "extracting audio")
}

func (e *VideoEditor) MergeAssetAndAudioFile() {
	savedDir := filepath.Join("files", e.VideoID)
	audioFile := filepath.Join(savedDir, "audio.mp3")
	finalOutput := filepath.Join(savedDir, "final_output.mp4")

	if _, err := os.Stat(finalOutput); err == nil {
		e.Logger.LogFileWithStdout("final_output.mp4 already exists. Skipping this step !!", logger.Info)
		return
	}

	e.Logger.LogFileWithStdout("Merging audio and asset file together", logger.Info)
	e.runFfmpeg([]string{
		"-i", e.ConfigLoader.ConfigData.AssetVideoPath,
		"-i", audioFile,
		"-c:v", "copy",
		"-c:a", "aac",
		finalOutput,
	}, "merging asset and audio")
}

func (e *VideoEditor) GetVideoDuration() {
	if e.Failed {
		return
	}
	savedDir := filepath.Join("files", e.VideoID)
	finalOutput := filepath.Join(savedDir, "final_output.mp4")

	e.Logger.LogFileWithStdout("Calculating video duration.", logger.Info)
	
	cmd := exec.Command(e.ConfigLoader.GetFfprobePath(), "-v", "quiet", "-show_entries", "format=duration", "-of", "csv=p=0", finalOutput)
	out, err := cmd.Output()
	if err != nil {
		e.Failed = true
		e.Logger.LogFileWithStdout("Failed Calculating video duration", logger.Error)
		return
	}
	
	durationStr := strings.TrimSpace(string(out))
	if f, err := strconv.ParseFloat(durationStr, 64); err == nil {
		e.FinalOutputVideoDuration = int(f)
		e.Logger.LogFileWithStdout(fmt.Sprintf("Calculated video duration :%d s", e.FinalOutputVideoDuration), logger.Info)
	}
}

func (e *VideoEditor) GenerateConcatDemuxerFile() {
	if e.Failed {
		return
	}

	savedDir := filepath.Join("files", e.VideoID)
	filesTxt := filepath.Join(savedDir, "files.txt")

	if _, err := os.Stat(filesTxt); err == nil {
		e.Logger.LogFileWithStdout("files.txt already exists. Skipping this step !!", logger.Info)
		return
	}

	e.Logger.LogFileWithStdout("Generating concat demuxer file.", logger.Info)

	outputDuration := float64(e.ConfigLoader.GetFinalVideoDuration())
	mergedDuration := float64(e.FinalOutputVideoDuration)
	if mergedDuration == 0 {
		mergedDuration = 30
	}
	repetitions := int(math.Ceil(outputDuration / mergedDuration))

	file, err := os.Create(filesTxt)
	if err != nil {
		e.Logger.LogFileWithStdout("Failed to generate demuxer file.", logger.Error)
		return
	}
	defer file.Close()

	finalOutputPath := filepath.Join(e.ConfigLoader.Pwd, savedDir, "final_output.mp4")
	
	// escape single quotes in path if necessary
	finalOutputPath = strings.ReplaceAll(finalOutputPath, "'", "'\\''")

	for i := 0; i < repetitions; i++ {
		file.WriteString(fmt.Sprintf("file '%s'\n", finalOutputPath))
	}
}

func (e *VideoEditor) RenderFinalOutputVideo() string {
	if e.Failed {
		return ""
	}

	title := e.GetVideoTitle()
	filename := e.GenerateOutputFilename(title)
	savedDir := filepath.Join("files", e.VideoID)
	filesTxt := filepath.Join(savedDir, "files.txt")
	outputDir := filepath.Join(e.ConfigLoader.ConfigData.OutputDirectory, e.VideoID)

	os.MkdirAll(outputDir, 0755)
	
	finalRender := filepath.Join(outputDir, filename+".mp4")
	if _, err := os.Stat(finalRender); err == nil {
		e.Logger.LogFileWithStdout("Output was already created, Skipping this step !", logger.Info)
		return finalRender
	}

	e.Logger.LogFileWithStdout("Rendering final video.", logger.Info)
	start := time.Now()
	e.runFfmpeg([]string{
		"-f", "concat",
		"-safe", "0",
		"-i", filesTxt,
		"-c", "copy",
		finalRender,
	}, "Rendering output file")
	
	if !e.Failed {
		e.Logger.LogFileWithStdout(fmt.Sprintf("Render Time Took : %v", time.Since(start)), logger.Info)
	}

	return finalRender
}

func (e *VideoEditor) DownloadOriginalThumbnail() string {
	outputDir := filepath.Join(e.ConfigLoader.ConfigData.OutputDirectory, e.VideoID)
	thumbPath := filepath.Join(outputDir, e.VideoID+".jpg")
	apiURL := fmt.Sprintf("https://i.ytimg.com/vi/%s/hqdefault.jpg", e.VideoID)

	resp, err := http.Get(apiURL)
	if err != nil || resp.StatusCode == 404 {
		e.Logger.LogFileWithStdout("Failed to request youtube for thumbnail.", logger.Error)
		return ""
	}
	defer resp.Body.Close()

	file, err := os.Create(thumbPath)
	if err != nil {
		e.Logger.LogFileWithStdout("Failed to save thumbnail file.", logger.Error)
		return ""
	}
	defer file.Close()

	if _, err := file.ReadFrom(resp.Body); err != nil {
		e.Logger.LogFileWithStdout("Failed writing thumbnail.", logger.Error)
		return ""
	}
	e.Logger.LogFileWithStdout("Successfully downloaded thumbnail file.", logger.Info)

	return thumbPath
}
