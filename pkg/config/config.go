package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"

	"gozart/pkg/logger"
)

type ConfigParams struct {
	FfmpegPath         string `json:"ffmpegPath"`
	YtdlpPath          string `json:"ytdlpPath"`
	FinalVideoDuration string `json:"final-video-duration"`
	YoutubeAPIKey      string `json:"youtube-api-key"`
	AssetVideoPath     string `json:"asset-video-path"`
	OutputDirectory    string `json:"output-directory"`
	LinksFilePath      string `json:"links-file-path"`
}

type ConfigLoader struct {
	Pwd        string
	ConfigData ConfigParams
	Logger     *logger.Logger
}

func New(l *logger.Logger) *ConfigLoader {
	pwd, err := os.Getwd()
	if err != nil {
		l.LogFileWithStdout("Failed to get working directory", logger.Fatal)
	}

    // Go up one directory if run inside gozart, since config.json is in parent dir
    // For simplicity, let's assume config.json is in the current working directory where the CLI is run.
	configPath := filepath.Join(pwd, "config.json")
	l.LogFileWithStdout("Searching config.json at " + configPath, logger.Info)

	file, err := os.ReadFile(configPath)
	if err != nil {
		l.LogFileWithStdout("Not found config.json: "+err.Error(), logger.Fatal)
	}

	var data ConfigParams
	if err := json.Unmarshal(file, &data); err != nil {
		l.LogFileWithStdout("Failed to parse config.json: "+err.Error(), logger.Fatal)
	}

	l.LogFileWithStdout("Found config.json", logger.Info)

	if data.OutputDirectory == "" {
		data.OutputDirectory = "output"
	}
	if data.LinksFilePath == "" {
		data.LinksFilePath = "links.txt"
	}
	if data.FfmpegPath == "" {
		data.FfmpegPath = "ffmpeg"
	}
	if data.YtdlpPath == "" {
		data.YtdlpPath = "yt-dlp"
	}
	if data.AssetVideoPath == "" {
		data.AssetVideoPath = "sample.mp4"
	}
	if data.FinalVideoDuration == "" {
		data.FinalVideoDuration = "30"
	}

	// Validate paths
	if _, err := os.Stat(data.AssetVideoPath); os.IsNotExist(err) {
		l.LogFileWithStdout("Asset video file not found.", logger.Fatal)
	} else {
		l.LogFileWithStdout("Asset video file found.", logger.Info)
	}

	if _, err := os.Stat(data.OutputDirectory); os.IsNotExist(err) {
		l.LogFileWithStdout("Output directory doesn't exist, Creating one..", logger.Error)
		os.MkdirAll(data.OutputDirectory, 0755)
		l.LogFileWithStdout("Created Output directory", logger.Info)
	} else {
		l.LogFileWithStdout("Output directory exists", logger.Info)
	}

	if _, err := os.Stat(data.LinksFilePath); os.IsNotExist(err) {
		l.LogFileWithStdout("Links file not found", logger.Fatal)
	} else {
		l.LogFileWithStdout("Links File exists", logger.Info)
	}

	if data.YoutubeAPIKey == "" {
		l.LogFileWithStdout("Not Found Api key, Please paste a youtube api key.", logger.Fatal)
	}

	return &ConfigLoader{
		Pwd:        pwd,
		ConfigData: data,
		Logger:     l,
	}
}

func (c *ConfigLoader) GetFinalVideoDuration() int {
	duration, err := strconv.Atoi(c.ConfigData.FinalVideoDuration)
	if err != nil {
		c.Logger.LogFileWithStdout("Enter a valid integer in config.json for final-video-duration", logger.Fatal)
	}
	return duration
}

func (c *ConfigLoader) GetFfprobePath() string {
	dir := filepath.Dir(c.ConfigData.FfmpegPath)
	if dir == "." || dir == "" {
		return "ffprobe"
	}
	return filepath.Join(dir, "ffprobe")
}

func (c *ConfigLoader) CheckForFfmpeg() {
	c.Logger.LogFileWithStdout("Searching for ffmpeg on your device.", logger.Info)
	cmd := exec.Command(c.ConfigData.FfmpegPath, "-version")
	if err := cmd.Run(); err != nil {
		c.Logger.LogFileWithStdout("ffmpeg check failed: "+err.Error(), logger.Fatal)
	}
	c.Logger.LogFileWithStdout("ffmpeg is installed", logger.Info)
}

func (c *ConfigLoader) CheckForYtdlp() {
	c.Logger.LogFileWithStdout("Searching for yt-dlp on your device.", logger.Info)
	cmd := exec.Command(c.ConfigData.YtdlpPath, "--version")
	if err := cmd.Run(); err != nil {
		c.Logger.LogFileWithStdout("yt-dlp check failed: "+err.Error(), logger.Fatal)
	}
	c.Logger.LogFileWithStdout("yt-dlp is installed", logger.Info)
}
