package sentiment

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"strings"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type CommentScraper struct {
	APIKey    string
	YtdlpPath string
}

func NewScraper(apiKey, ytdlpPath string) *CommentScraper {
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}
	return &CommentScraper{
		APIKey:    apiKey,
		YtdlpPath: ytdlpPath,
	}
}

// FetchTopComments fetches up to maxComments for a given video ID using API or yt-dlp.
func (cs *CommentScraper) FetchTopComments(ctx context.Context, videoID string, maxComments int64) ([]string, error) {
	if maxComments <= 0 {
		maxComments = 50
	}

	var comments []string
	var err error

	if cs.APIKey != "" {
		comments, err = cs.fetchWithAPI(ctx, videoID, maxComments)
	}

	if len(comments) == 0 || err != nil {
		fallbackComments, fallbackErr := cs.fetchWithYtDlp(ctx, videoID, maxComments)
		if fallbackErr == nil && len(fallbackComments) > 0 {
			comments = fallbackComments
			err = nil
		}
	}

	if err != nil && len(comments) == 0 {
		return nil, fmt.Errorf("failed to fetch comments for video %s: %w", videoID, err)
	}

	return comments, nil
}

func (cs *CommentScraper) fetchWithAPI(ctx context.Context, videoID string, maxComments int64) ([]string, error) {
	service, err := youtube.NewService(ctx, option.WithAPIKey(cs.APIKey))
	if err != nil {
		return nil, err
	}

	call := service.CommentThreads.List([]string{"snippet"}).
		VideoId(videoID).
		Order("relevance").
		TextFormat("plainText").
		MaxResults(maxComments)

	resp, err := call.Do()
	if err != nil {
		return nil, err
	}

	var comments []string
	for _, item := range resp.Items {
		if item.Snippet != nil && item.Snippet.TopLevelComment != nil && item.Snippet.TopLevelComment.Snippet != nil {
			text := strings.TrimSpace(item.Snippet.TopLevelComment.Snippet.TextDisplay)
			if text != "" {
				comments = append(comments, text)
			}
		}
	}
	return comments, nil
}

func (cs *CommentScraper) fetchWithYtDlp(ctx context.Context, videoID string, maxComments int64) ([]string, error) {
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)
	cmd := exec.CommandContext(ctx, cs.YtdlpPath,
		"--skip-download",
		"--write-comments",
		"--dump-single-json",
		videoURL,
	)

	stdout, err := cmd.StdoutPipe()
	if err != nil {
		return nil, err
	}
	if err := cmd.Start(); err != nil {
		return nil, err
	}

	type YtComment struct {
		Text string `json:"text"`
	}
	type YtVideoJSON struct {
		Comments []YtComment `json:"comments"`
	}

	var parsed YtVideoJSON
	scanner := bufio.NewScanner(stdout)
	buf := make([]byte, 1024*1024)
	scanner.Buffer(buf, 10*1024*1024) // accommodate large JSON dumps

	var allOutput strings.Builder
	for scanner.Scan() {
		allOutput.WriteString(scanner.Text())
	}
	_ = cmd.Wait()

	if err := json.Unmarshal([]byte(allOutput.String()), &parsed); err == nil && len(parsed.Comments) > 0 {
		var res []string
		for i, c := range parsed.Comments {
			if int64(i) >= maxComments {
				break
			}
			text := strings.TrimSpace(c.Text)
			if text != "" {
				res = append(res, text)
			}
		}
		return res, nil
	}

	return nil, fmt.Errorf("no comments parsed via yt-dlp")
}
