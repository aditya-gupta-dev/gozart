package cmd

import (
	"context"
	"fmt"
	"os"
	"time"

	"gozart/pkg/config"
	"gozart/pkg/logger"
	"gozart/pkg/pipeline"
	"gozart/pkg/storage"

	"github.com/spf13/cobra"
)

var (
	topicFlag       string
	countryFlag     string
	formatFlag      string
	durationFlag    int
	toneFlag        string
	modelTierFlag   string
	autoPublishFlag bool
)

var createCmd = &cobra.Command{
	Use:   "create",
	Short: "Run the autonomous AI video generation pipeline from CLI",
	Long:  `Conducts competitive research, sentiment analysis, scriptwriting, voiceover, media rendering, and SEO from a single topic.`,
	Run: func(cmd *cobra.Command, args []string) {
		if topicFlag == "" {
			fmt.Println("Error: --topic is required")
			os.Exit(1)
		}

		l := logger.New()
		defer l.Close()

		cfg := config.New(l)
		db, err := storage.New(cfg.ConfigData.Storage.DBPath)
		if err != nil {
			l.LogFileWithStdout(fmt.Sprintf("Failed to initialize database: %v", err), logger.Fatal)
			os.Exit(1)
		}
		defer db.Close()

		orch := pipeline.NewOrchestrator(db, cfg, func(jobID, level, msg string) {
			l.LogFileWithStdout(fmt.Sprintf("[%s] %s", level, msg), logger.Info)
		})

		jobID := fmt.Sprintf("%d", time.Now().Unix())
		params := pipeline.PipelineJobParams{
			JobID:             jobID,
			Topic:             topicFlag,
			CountryCode:       countryFlag,
			Format:            formatFlag,
			Tone:              toneFlag,
			TargetDurationSec: durationFlag,
			ModelTier:         modelTierFlag,
			AutoUpload:        autoPublishFlag,
		}

		l.LogFileWithStdout(fmt.Sprintf("Starting autonomous job %s for '%s'...", jobID, topicFlag), logger.Info)
		if err := orch.Run(context.Background(), params); err != nil {
			l.LogFileWithStdout(fmt.Sprintf("Pipeline failed: %v", err), logger.Fatal)
		}
		l.LogFileWithStdout(fmt.Sprintf("Job %s completed successfully!", jobID), logger.Info)
	},
}

func init() {
	rootCmd.AddCommand(createCmd)
	createCmd.Flags().StringVarP(&topicFlag, "topic", "t", "", "Video topic or premise (required)")
	createCmd.Flags().StringVarP(&countryFlag, "country", "r", "US", "2-letter ISO country code (e.g. US, GB, IN)")
	createCmd.Flags().StringVarP(&formatFlag, "format", "f", "shorts_9_16", "Target format: shorts_9_16 or longform_16_9")
	createCmd.Flags().IntVarP(&durationFlag, "duration", "d", 50, "Target duration in seconds")
	createCmd.Flags().StringVar(&toneFlag, "tone", "sensational", "Tone: sensational, educational, storytelling, humorous, mystery")
	createCmd.Flags().StringVarP(&modelTierFlag, "model-tier", "m", "local", "Model tier: local (Ollama) or cloud (OpenAI/Cloud)")
	createCmd.Flags().BoolVarP(&autoPublishFlag, "upload", "u", false, "Auto-publish to YouTube upon completion")
}
