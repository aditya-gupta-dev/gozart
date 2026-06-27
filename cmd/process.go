package cmd

import (
	"path/filepath"
	"sync"

	"github.com/spf13/cobra"
	"gozart/pkg/config"
	"gozart/pkg/downloader"
	"gozart/pkg/editor"
	"gozart/pkg/logger"
	"gozart/pkg/uploader"
)

var (
	uploadFlag      bool
	concurrencyFlag int
)

var processCmd = &cobra.Command{
	Use:   "process",
	Short: "Download, edit, and optionally upload videos",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.New()
		defer l.Close()

		cfg := config.New(l)
		cfg.CheckForFfmpeg()
		cfg.CheckForYtdlp()

		var ytUploader *uploader.YouTubeUploader
		if uploadFlag {
			ytUploader = uploader.New(l, cfg)
			ytUploader.Authenticate()
		}

		dl := downloader.New(l, cfg)
		links := dl.GetLinksFromFile()

		var wg sync.WaitGroup
		sem := make(chan struct{}, concurrencyFlag)

		for _, link := range links {
			if link == "" {
				continue
			}

			wg.Add(1)
			sem <- struct{}{} // acquire

			go func(videoLink string) {
				defer wg.Done()
				defer func() { <-sem }() // release

				videoID := dl.DownloadVideoUsingPkg(videoLink)
				if videoID == "" {
					return
				}

				ed := editor.New(videoLink, l, cfg, videoID)
				ed.Edit()

				if uploadFlag && !ed.Failed {
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
		if uploadFlag && ytUploader != nil {
			ytUploader.Wait()
		}
		l.LogFileWithStdout("All processing completed.", logger.Info)
	},
}

func init() {
	rootCmd.AddCommand(processCmd)
	processCmd.Flags().BoolVarP(&uploadFlag, "upload", "u", false, "Actually upload the video to YouTube")
	processCmd.Flags().IntVarP(&concurrencyFlag, "concurrency", "c", 2, "Number of videos to process concurrently")
}
