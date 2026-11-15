package downloader

import (
	"os"
	"path/filepath"
	"testing"

	"gozart/pkg/config"
	"gozart/pkg/logger"
)

func TestDownloadCache(t *testing.T) {
	const videoID = "abcdefghijk"
	for _, tc := range []struct {
		name       string
		cachedFile string
		content    string
		download   bool
	}{
		{"cached webm", "input.webm", "cached video", false},
		{"cached mp4", "input.mp4", "cached video", false},
		{"missing cache", "", "", true},
		{"empty cached video", "input.webm", "", true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			root := t.TempDir()
			t.Chdir(root)
			t.Setenv("HOME", filepath.Join(root, "home"))
			cacheDir := filepath.Join(root, "home", ".cache", "gozart", videoID)
			if tc.cachedFile != "" {
				if err := os.MkdirAll(cacheDir, 0755); err != nil {
					t.Fatal(err)
				}
				if err := os.WriteFile(filepath.Join(cacheDir, tc.cachedFile), []byte(tc.content), 0644); err != nil {
					t.Fatal(err)
				}
			}
			// An existing work directory must not cause a nested video-ID folder.
			if err := os.MkdirAll(filepath.Join("files", videoID), 0755); err != nil {
				t.Fatal(err)
			}
			fakeDownloader := filepath.Join(root, "yt-dlp")
			if err := os.WriteFile(fakeDownloader, []byte("#!/bin/sh\nset -e\nprintf called >> download-calls\nprintf 'downloaded video' > files/abcdefghijk/input.webm\n"), 0755); err != nil {
				t.Fatal(err)
			}
			l := logger.New()
			defer l.Close()
			cfg := &config.ConfigLoader{}
			cfg.ConfigData.YtdlpPath = fakeDownloader
			d := &VideoDownloader{logger: l, configLoader: cfg, tempDir: "files"}
			if got := d.DownloadVideoUsingPkg("https://youtu.be/" + videoID); got != videoID {
				t.Fatalf("got video ID %q, want %q", got, videoID)
			}
			_, err := os.Stat("download-calls")
			if tc.download && err != nil {
				t.Fatalf("expected download: %v", err)
			}
			if !tc.download && !os.IsNotExist(err) {
				t.Fatalf("cached video triggered download: %v", err)
			}
			name, want := tc.cachedFile, tc.content
			if tc.download {
				name, want = "input.webm", "downloaded video"
			}
			for _, dir := range []string{filepath.Join("files", videoID), cacheDir} {
				got, err := os.ReadFile(filepath.Join(dir, name))
				if err != nil || string(got) != want {
					t.Fatalf("video in %s: got %q, err %v; want %q", dir, got, err, want)
				}
			}
		})
	}
}
