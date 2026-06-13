package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gozart",
	Short: "Gozart is a tool for rendering and uploading looped videos",
	Long: `Gozart automates the downloading, processing, and uploading of YouTube videos.
It fetches videos, extracts audio, merges with an asset video, loops it, and uploads it.`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
