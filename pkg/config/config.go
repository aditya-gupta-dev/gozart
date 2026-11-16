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

type ServerConfig struct {
	Port int    `json:"port"`
	Host string `json:"host"`
}

type AIConfig struct {
	Provider      string `json:"provider"`
	OllamaBaseURL string `json:"ollama_base_url"`
	AgentModel    string `json:"agent_model"`
	FastModel     string `json:"fast_model"`
	APIKey        string `json:"api_key"`
}

type YouTubeConfig struct {
	APIKey            string `json:"api_key"`
	ClientSecretsPath string `json:"client_secrets_path"`
	TokenCachePath    string `json:"token_cache_path"`
}

type AudioConfig struct {
	TTSEngine         string `json:"tts_engine"`
	Voice             string `json:"voice"`
	DeepFilterEnabled bool   `json:"deepfilter_enabled"`
	DeepFilterPath    string `json:"deepfilter_path"`
}

type MediaConfig struct {
	Engine            string `json:"engine"`
	NodeBinary        string `json:"node_binary"`
	MaxHealingRetries int    `json:"max_healing_retries"`
}

type StorageConfig struct {
	DBPath    string `json:"db_path"`
	WorkDir   string `json:"workdir"`
	OutputDir string `json:"output_dir"`
}

type ConfigParams struct {
	// New structured configuration
	Server  ServerConfig  `json:"server"`
	AI      AIConfig      `json:"ai"`
	YouTube YouTubeConfig `json:"youtube"`
	Audio   AudioConfig   `json:"audio"`
	Media   MediaConfig   `json:"media"`
	Storage StorageConfig `json:"storage"`

	// Legacy flat fields preserved for backward compatibility
	FfmpegPath          string `json:"ffmpegPath"`
	YtdlpPath           string `json:"ytdlpPath"`
	FinalVideoDuration  string `json:"final-video-duration"`
	YoutubeAPIKey       string `json:"youtube-api-key"`
	AssetVideoPath      string `json:"asset-video-path"`
	OutputDirectory     string `json:"output-directory"`
	LinksFilePath       string `json:"links-file-path"`
	DescriptionFilePath string `json:"description-file-path"`
	TagsFilePath        string `json:"tags-file-path"`
	AIAuthToken         string `json:"ai-auth-token"`
	ModelTag            string `json:"model-tag"`
}

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

	configPath := filepath.Join(pwd, "config.json")
	l.LogFileWithStdout("Searching config.json at "+configPath, logger.Info)

	var data ConfigParams
	file, err := os.ReadFile(configPath)
	if err != nil {
		l.LogFileWithStdout("config.json not found or not readable: "+err.Error()+", using defaults", logger.Warn)
	} else {
		if err := json.Unmarshal(file, &data); err != nil {
			l.LogFileWithStdout("Failed to parse config.json: "+err.Error(), logger.Fatal)
		}
		l.LogFileWithStdout("Loaded config.json", logger.Info)
	}

	// Apply defaults and bridge structured <-> legacy fields
	if data.Server.Port == 0 {
		data.Server.Port = 8080
	}
	if data.Server.Host == "" {
		data.Server.Host = "0.0.0.0"
	}

	if data.AI.Provider == "" {
		data.AI.Provider = "ollama"
	}
	if data.AI.OllamaBaseURL == "" {
		data.AI.OllamaBaseURL = "http://localhost:11434"
	}
	if data.AI.AgentModel == "" {
		data.AI.AgentModel = "qwen2.5:32b"
	}
	if data.AI.FastModel == "" {
		data.AI.FastModel = "qwen2.5:7b"
	}

	// Bridge YouTube API key
	if data.YouTube.APIKey == "" && data.YoutubeAPIKey != "" {
		data.YouTube.APIKey = data.YoutubeAPIKey
	} else if data.YoutubeAPIKey == "" && data.YouTube.APIKey != "" {
		data.YoutubeAPIKey = data.YouTube.APIKey
	}
	if data.YouTube.ClientSecretsPath == "" {
		data.YouTube.ClientSecretsPath = "client_secrets.json"
	}
	if data.YouTube.TokenCachePath == "" {
		data.YouTube.TokenCachePath = "token.json"
	}

	// Audio defaults
	if data.Audio.TTSEngine == "" {
		data.Audio.TTSEngine = "kokoro"
	}
	if data.Audio.Voice == "" {
		data.Audio.Voice = "af_bella"
	}
	if data.Audio.DeepFilterPath == "" {
		data.Audio.DeepFilterPath = "deep-filter"
	}

	// Media defaults
	if data.Media.Engine == "" {
		data.Media.Engine = "remotion"
	}
	if data.Media.NodeBinary == "" {
		data.Media.NodeBinary = "node"
	}
	if data.Media.MaxHealingRetries <= 0 {
		data.Media.MaxHealingRetries = 3
	}

	// Storage defaults
	if data.Storage.DBPath == "" {
		data.Storage.DBPath = "gozart.db"
	}
	if data.Storage.WorkDir == "" {
		data.Storage.WorkDir = "workspace"
	}
	if data.Storage.OutputDir == "" {
		if data.OutputDirectory != "" {
			data.Storage.OutputDir = data.OutputDirectory
		} else {
			data.Storage.OutputDir = "output"
		}
	}
	if data.OutputDirectory == "" {
		data.OutputDirectory = data.Storage.OutputDir
	}

	// Legacy defaults
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

	// Environment overrides (used by the Docker image)
	if v := os.Getenv("GOZART_DB_PATH"); v != "" {
		data.Storage.DBPath = v
	}
	if v := os.Getenv("GOZART_OLLAMA_URL"); v != "" {
		data.AI.OllamaBaseURL = v
	}

	// Ensure required directories exist
	if err := os.MkdirAll(data.Storage.OutputDir, 0755); err != nil {
		l.LogFileWithStdout("Failed to create output directory: "+err.Error(), logger.Warn)
	}
	if err := os.MkdirAll(data.Storage.WorkDir, 0755); err != nil {
		l.LogFileWithStdout("Failed to create work directory: "+err.Error(), logger.Warn)
	}

	return &ConfigLoader{
		Pwd:        pwd,
		ConfigData: data,
		Logger:     l,
	}
}

// SaveConfig writes the current configuration data to config.json in Pwd.
func (c *ConfigLoader) SaveConfig() error {
	configPath := filepath.Join(c.Pwd, "config.json")
	data, err := json.MarshalIndent(c.ConfigData, "", "  ")
	if err != nil {
		return err
	}
	return os.WriteFile(configPath, data, 0644)
}

// ValidateLegacyInputs verifies legacy video asset and link files exist for loop processing.
func (c *ConfigLoader) ValidateLegacyInputs() {
	if _, err := os.Stat(c.ConfigData.AssetVideoPath); os.IsNotExist(err) {
		c.Logger.LogFileWithStdout("Asset video file not found: "+c.ConfigData.AssetVideoPath, logger.Fatal)
	}
	if _, err := os.Stat(c.ConfigData.LinksFilePath); os.IsNotExist(err) {
		c.Logger.LogFileWithStdout("Links file not found: "+c.ConfigData.LinksFilePath, logger.Fatal)
	}
	if c.ConfigData.YoutubeAPIKey == "" {
		c.Logger.LogFileWithStdout("YouTube API key not found in config", logger.Fatal)
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
