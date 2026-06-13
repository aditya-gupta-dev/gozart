package downloader

import (
	"os"
	"os/exec"
	"regexp"

	"gozart/pkg/config"
	"gozart/pkg/logger"
)

type VideoDownloader struct {
	logger       *logger.Logger
	configLoader *config.ConfigLoader
	tempDir      string
}

func New(l *logger.Logger, cfg *config.ConfigLoader) *VideoDownloader {
	tempDir := "files"
	if err := os.MkdirAll(cfg.ConfigData.OutputDirectory, 0755); err != nil {
		l.LogFileWithStdout("Error Occured "+err.Error(), logger.Fatal)
	}

	if err := os.MkdirAll(tempDir, 0755); err != nil {
		l.LogFileWithStdout("Error Occured "+err.Error(), logger.Fatal)
	}

	return &VideoDownloader{
		logger:       l,
		configLoader: cfg,
		tempDir:      tempDir,
	}
}

func (d *VideoDownloader) GetVideoID(url string) string {
	d.logger.LogFileOnly("parsing link "+url, logger.Info)
	re := regexp.MustCompile(`(?:youtu\.be\/|youtube\.com\/(?:.*v=|.*\/|.*embed\/|v\/|shorts\/))([\w-]{11})`)
	matches := re.FindStringSubmatch(url)
	if len(matches) > 1 {
		d.logger.LogFileOnly("parsed link "+url+" -> Result "+matches[1], logger.Info)
		return matches[1]
	}
	return ""
}

func (d *VideoDownloader) GetLinksFromFile() []string {
	content, err := os.ReadFile(d.configLoader.ConfigData.LinksFilePath)
	if err != nil {
		d.logger.LogFileWithStdout("Error reading links file: "+err.Error(), logger.Fatal)
	}

	re := regexp.MustCompile(`\r?\n`)
	lines := re.Split(string(content), -1)
	
	var links []string
	for _, line := range lines {
		if line != "" {
			links = append(links, line)
		}
	}
	return links
}

func (d *VideoDownloader) DownloadVideoUsingPkg(link string) string {
	videoID := d.GetVideoID(link)
	if videoID == "" {
		d.logger.LogFileWithStdout("Invalid video ID for link: "+link, logger.Error)
		return ""
	}

	savePath := d.tempDir + "/" + videoID + "/input.webm"
	altPath := d.tempDir + "/" + videoID + "/input.mp4"

	if _, err := os.Stat(savePath); err == nil {
		d.logger.LogFileWithStdout("downloaded video is already present. Skipping Downloading...", logger.Info)
		return videoID
	}
	if _, err := os.Stat(altPath); err == nil {
		d.logger.LogFileWithStdout("downloaded video is already present. Skipping Downloading...", logger.Info)
		return videoID
	}

	d.logger.LogFileWithStdout("Started Downloading "+link, logger.Info)
	
	cmd := exec.Command(d.configLoader.ConfigData.YtdlpPath, link, "--output", d.tempDir+"/%(id)s/input.%(ext)s")
	if err := cmd.Run(); err != nil {
		d.logger.LogFileWithStdout("Failed to download the video "+link, logger.Error)
		return ""
	}

	d.logger.LogFileWithStdout("Completed Downloading "+link, logger.Info)
	return videoID
}
