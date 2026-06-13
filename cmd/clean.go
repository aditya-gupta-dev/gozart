package cmd

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
	"gozart/pkg/config"
	"gozart/pkg/logger"
)

var resetFlag bool

var cleanCmd = &cobra.Command{
	Use:   "clean",
	Short: "Clean output and temporary files",
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.New()
		defer l.Close()

		cfg := config.New(l)
		l.LogFileWithStdout("Starting cleaner", logger.Info)

		if resetFlag {
			if err := os.Remove(cfg.ConfigData.AssetVideoPath); err == nil {
				l.LogFileWithStdout("Removed asset video path", logger.Info)
			}
			if err := os.WriteFile(cfg.ConfigData.LinksFilePath, []byte(""), 0644); err == nil {
				l.LogFileWithStdout("Emptied links file", logger.Info)
			}
		}

		filesDir := "files"
		if err := os.RemoveAll(cfg.ConfigData.OutputDirectory); err == nil {
			l.LogFileWithStdout("Removed output folder", logger.Info)
		}
		if err := os.RemoveAll(filesDir); err == nil {
			l.LogFileWithStdout("Removed files folder", logger.Info)
		}

		entries, _ := os.ReadDir(".")
		count := 0
		for _, entry := range entries {
			if filepath.Ext(entry.Name()) == ".log" && !entry.IsDir() {
				if err := os.Remove(entry.Name()); err == nil {
					count++
				}
			}
		}
		l.LogFileWithStdout(fmt.Sprintf("Removed a total of %d log files", count), logger.Info)
	},
}

func init() {
	rootCmd.AddCommand(cleanCmd)
	cleanCmd.Flags().BoolVar(&resetFlag, "reset", false, "Reset asset video and links file")
}
