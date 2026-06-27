package config

import (
	"encoding/json"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	"gozart/pkg/logger"
)

type ConfigParams struct {
	FfmpegPath          string `json:"ffmpegPath"`
	YtdlpPath           string `json:"ytdlpPath"`
	FinalVideoDuration  string `json:"final-video-duration"`
	YoutubeAPIKey       string `json:"youtube-api-key"`
	AssetVideoPath      string `json:"asset-video-path"`
	OutputDirectory     string `json:"output-directory"`
	LinksFilePath       string `json:"links-file-path"`
	DescriptionFilePath string `json:"description-file-path"`
	TagsFilePath        string `json:"tags-file-path"`
}

// Fallbacks used when no description/tags file is configured or readable.
const defaultDescription = "Open for promoting all kinds of music. Hit me up, Contact details provided in the channel's about page"

var defaultTags = []string{"shorts", "music", "looped"}

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
	l.LogFileWithStdout("Searching config.json at "+configPath, logger.Info)

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

	// Description/tags files are optional. Warn (non-fatal) if a path is set
	// but missing, so uploads silently falling back to defaults is visible.
	if data.DescriptionFilePath != "" {
		if _, err := os.Stat(data.DescriptionFilePath); os.IsNotExist(err) {
			l.LogFileWithStdout("Description file not found, will use default description.", logger.Error)
		} else {
			l.LogFileWithStdout("Description file found.", logger.Info)
		}
	}

	if data.TagsFilePath != "" {
		if _, err := os.Stat(data.TagsFilePath); os.IsNotExist(err) {
			l.LogFileWithStdout("Tags file not found, will use default tags.", logger.Error)
		} else {
			l.LogFileWithStdout("Tags file found.", logger.Info)
		}
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

// GetDescription returns the video description read from the file configured
// via "description-file-path". If the path is unset, the file is unreadable,
// or its contents are empty, it falls back to the default description.
func (c *ConfigLoader) GetDescription() string {
	path := c.ConfigData.DescriptionFilePath
	if path == "" {
		return defaultDescription
	}

	data, err := os.ReadFile(path)
	if err != nil {
		c.Logger.LogFileWithStdout("Unable to read description file ("+path+"): "+err.Error()+". Using default description.", logger.Error)
		return defaultDescription
	}

	desc := strings.TrimSpace(string(data))
	if desc == "" {
		c.Logger.LogFileWithStdout("Description file is empty, using default description.", logger.Error)
		return defaultDescription
	}
	return desc
}

// GetTags returns the video tags read from the file configured via
// "tags-file-path". Tags may be separated by newlines and/or commas. If the
// path is unset, the file is unreadable, or no tags are found, it falls back
// to the default tags.
func (c *ConfigLoader) GetTags() []string {
	path := c.ConfigData.TagsFilePath
	if path == "" {
		return defaultTags
	}

	data, err := os.ReadFile(path)
	if err != nil {
		c.Logger.LogFileWithStdout("Unable to read tags file ("+path+"): "+err.Error()+". Using default tags.", logger.Error)
		return defaultTags
	}

	fields := strings.FieldsFunc(string(data), func(r rune) bool {
		return r == '\n' || r == '\r' || r == ','
	})

	var tags []string
	for _, f := range fields {
		if t := strings.TrimSpace(f); t != "" {
			tags = append(tags, t)
		}
	}

	if len(tags) == 0 {
		c.Logger.LogFileWithStdout("Tags file contained no tags, using default tags.", logger.Error)
		return defaultTags
	}
	return tags
}
