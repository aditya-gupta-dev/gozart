package cmd

import (
	"fmt"
	"gozart/utils"
	"io/fs"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"
)

var cacheCommand = &cobra.Command{
	Use:   "cache",
	Short: "View Cache data stored by gozart.",
	Run: func(cmd *cobra.Command, args []string) {
		var totalSize float64 = 0
		fmt.Printf("\tGozart Cache Stats\n\n")

		var paths []string = []string{
			filepath.Join(utils.Unwrap(os.UserHomeDir()), ".cache/gozart"),
			filepath.Join(utils.Unwrap(os.Getwd()), "outpu"),
			filepath.Join(utils.Unwrap(os.Getwd()), "files"),
		}

		for _, path := range paths {
			size, err := getTotalSize(path)
			if err != nil {
				fmt.Printf("Error: unable to read ( %s ) : err-msg [ %s ]\n", path, err)
				continue
			}
			fmt.Println(path, "->", utils.FormatSize(float64(size)))
			totalSize += float64(size)
		}

		fmt.Printf("\nTotal Size: %s (%.3f MB)\n", utils.FormatSize(totalSize), totalSize/(1024*1024))
	},
}

func getTotalSize(path string) (int64, error) {
	var size int64 = 0
	err := filepath.WalkDir(path, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		size += utils.Unwrap(os.Stat(path)).Size()
		return nil
	})
	return size, err
}

func init() {
	rootCmd.AddCommand(cacheCommand)
}
