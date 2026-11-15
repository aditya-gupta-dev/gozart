package downloader

import (
	"os"
	"os/exec"
	"path/filepath"
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

	tempDirID := filepath.Join(d.tempDir, videoID)
	if hasDownloadedVideo(tempDirID) {
		d.logger.LogFileWithStdout("downloaded video is already present in files. Skipping Downloading...", logger.Info)
		return videoID
	}

	homeDir, err := os.UserHomeDir()
	if err != nil {
		d.logger.LogFileWithStdout("failed to get home directory for user.", logger.Fatal)
		return videoID
	}

	cacheRoot := filepath.Join(homeDir, ".cache", "gozart")
	cacheDir := filepath.Join(cacheRoot, videoID)
	if hasDownloadedVideo(cacheDir) {
		d.logger.LogFileWithStdout("downloaded video found in cache. Copying to files...", logger.Info)
		if err := copyVideoDirectory(cacheDir, tempDirID); err != nil {
			d.logger.LogFileWithStdout("Failed to restore cached video: "+err.Error(), logger.Error)
			return ""
		}
		if hasDownloadedVideo(tempDirID) {
			return videoID
		}
		d.logger.LogFileWithStdout("Restored cache contains no usable video", logger.Error)
		return ""
	}

	d.logger.LogFileWithStdout("Started Downloading "+link, logger.Info)

	cmd := exec.Command(d.configLoader.ConfigData.YtdlpPath, link, "--output", d.tempDir+"/%(id)s/input.%(ext)s")
	if err := cmd.Run(); err != nil {
		d.logger.LogFileWithStdout("Failed to download the video "+link, logger.Error)
		return ""
	}
	if !hasDownloadedVideo(tempDirID) {
		d.logger.LogFileWithStdout("Download produced no usable input.webm or input.mp4 for "+link, logger.Error)
		return ""
	}

	d.logger.LogFileWithStdout("Copying downloaded video to cache...", logger.Info)
	if err := copyVideoDirectory(tempDirID, cacheDir); err != nil {
		d.logger.LogFileWithStdout("Failed to cache downloaded video: "+err.Error(), logger.Warn)
	}

	d.logger.LogFileWithStdout("Completed Downloading "+link, logger.Info)
	return videoID
}

func hasDownloadedVideo(dir string) bool {
	for _, name := range []string{"input.webm", "input.mp4"} {
		info, err := os.Stat(filepath.Join(dir, name))
		if err == nil && info.Mode().IsRegular() && info.Size() > 0 {
			return true
		}
	}
	return false
}

func copyVideoDirectory(src, dst string) error {
	if err := os.MkdirAll(dst, 0755); err != nil {
		return err
	}
	// Copy contents into the destination, including when it already exists.
	return exec.Command("cp", "-r", filepath.Clean(src)+string(os.PathSeparator)+".", dst).Run()
}
