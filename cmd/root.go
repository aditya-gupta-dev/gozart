package cmd

import (
	"os"

	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "gozart",
	Short: "Gozart: Autonomous AI YouTube Video Generation Engine",
	Long: `Gozart is an end-to-end autonomous YouTube creation and distribution platform.
Given only a video topic and target audience, Gozart conducts competitive research,
mines audience comments for sentiment, synthesizes high-retention scripts,
generates voiceovers and Remotion visuals in parallel, polishes audio,
produces high-CTR thumbnails, optimizes SEO, and hosts a real-time web dashboard.

Website: https://ctoadi.web.app/
GitHub: https://github.com/aditya-gupta-dev/gozart.git`,
}

func Execute() {
	if err := rootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}
