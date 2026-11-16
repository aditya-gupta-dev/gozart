package cmd

import (
	"fmt"
	"os"

	"gozart/pkg/config"
	"gozart/pkg/logger"
	"gozart/pkg/storage"
	"gozart/web/handlers"

	"github.com/spf13/cobra"
)

var (
	webPortFlag int
	webHostFlag string
)

var webCmd = &cobra.Command{
	Use:   "web",
	Short: "Start the Gozart autonomous video engine web dashboard",
	Long:  `Launches the self-hosted Go templates + HTMX web dashboard for autonomous research, monitoring, and publishing.`,
	Run: func(cmd *cobra.Command, args []string) {
		l := logger.New()
		defer l.Close()

		cfg := config.New(l)
		if webPortFlag > 0 {
			cfg.ConfigData.Server.Port = webPortFlag
		}
		if webHostFlag != "" {
			cfg.ConfigData.Server.Host = webHostFlag
		}

		db, err := storage.New(cfg.ConfigData.Storage.DBPath)
		if err != nil {
			l.LogFileWithStdout(fmt.Sprintf("Failed to initialize database: %v", err), logger.Fatal)
			os.Exit(1)
		}
		defer db.Close()

		if err := db.SeedDemo(storage.DemoDir()); err != nil {
			l.LogFileWithStdout(fmt.Sprintf("Demo seed skipped: %v", err), logger.Warn)
		}

		server := handlers.NewServer(cfg, db)
		if err := server.Start(); err != nil {
			l.LogFileWithStdout(fmt.Sprintf("Server stopped: %v", err), logger.Fatal)
		}
	},
}

func init() {
	rootCmd.AddCommand(webCmd)
	webCmd.Flags().IntVarP(&webPortFlag, "port", "p", 8080, "Web server port")
	webCmd.Flags().StringVarP(&webHostFlag, "host", "H", "0.0.0.0", "Web server host interface")
}
