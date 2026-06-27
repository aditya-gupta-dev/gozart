package cmd

import (
	"fmt"
	"math/rand"
	"os"
	"path/filepath"
	"regexp"
	"sync"
	"time"

	"github.com/spf13/cobra"
	"gozart/pkg/config"
	"gozart/pkg/editor"
	"gozart/pkg/logger"
	"gozart/pkg/uploader"
)

var mode string

var randomizeCmd = &cobra.Command{
	Use:   "randomize",
	Short: "Randomly process videos from the temp directory",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.New()
		defer l.Close()

		cfg := config.New(l)
		cfg.CheckForFfmpeg()
		cfg.CheckForYtdlp()

		tempDir := "files"
		entries, err := os.ReadDir(tempDir)
		if err != nil {
			l.LogFileWithStdout("Error reading temp dir: "+err.Error(), logger.Fatal)
		}

		var allVideos []string
		for _, entry := range entries {
			if entry.IsDir() {
				allVideos = append(allVideos, "https://youtu.be/"+entry.Name())
			}
		}

		if len(allVideos) == 0 {
			l.LogFileWithStdout("No videos found in "+tempDir, logger.Info)
			return
		}

		// Initialize random locally to avoid using deprecated global Seed
		rng := rand.New(rand.NewSource(time.Now().UnixNano()))
		var selectedVideos []string

		switch mode {
		case "all":
			selectedVideos = allVideos
		case "few":
			count := rng.Intn(len(allVideos))
			if count == 0 {
				count = 1
			}
			for i := 0; i < count; i++ {
				selectedVideos = append(selectedVideos, allVideos[rng.Intn(len(allVideos))])
			}
		default: // "one"
			selectedVideos = []string{allVideos[rng.Intn(len(allVideos))]}
		}

		l.LogFileWithStdout("Selected Mode: "+mode, logger.Info)
		l.LogFileWithStdout(fmt.Sprintf("Proceeding with videos: %v", selectedVideos), logger.Info)

		// process randomly selected videos
		var wg sync.WaitGroup
		sem := make(chan struct{}, 2) // hardcoded concurrency 2 for randomize
		
		for _, link := range selectedVideos {
			wg.Add(1)
			sem <- struct{}{}
			go func(videoLink string) {
				defer wg.Done()
				defer func() { <-sem }()
				
				// parse video id from link
				re := regexp.MustCompile(`(?:youtu\.be\/|youtube\.com\/(?:.*v=|.*\/|.*embed\/|v\/|shorts\/))([\w-]{11})`)
				matches := re.FindStringSubmatch(videoLink)
				if len(matches) < 2 {
					return
				}
				videoID := matches[1]

				ed := editor.New(videoLink, l, cfg, videoID)
				ed.Edit()

				ytUploader := uploader.New(l, cfg)
				ytUploader.Authenticate()
				
				if !ed.Failed {
					title := ed.GetVideoTitle()
					filename := ed.GenerateOutputFilename(title)
					finalRender := filepath.Join(cfg.ConfigData.OutputDirectory, videoID, filename+".mp4")
					thumbPath := filepath.Join(cfg.ConfigData.OutputDirectory, videoID, videoID+".jpg")

					desc := cfg.GetDescription()
					uploadedID := ytUploader.UploadVideo(finalRender, title, desc, cfg.GetTags())
					if uploadedID != "" {
						ytUploader.UploadThumbnail(uploadedID, thumbPath)
					}
				}
			}(link)
		}
		wg.Wait()
		l.LogFileWithStdout("Randomize processing completed.", logger.Info)
	},
}

func init() {
	rootCmd.AddCommand(randomizeCmd)
	randomizeCmd.Flags().StringVar(&mode, "mode", "one", "Mode of randomizer: all, few, one")
}
