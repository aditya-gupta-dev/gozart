package research

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
	"time"

	"google.golang.org/api/option"
	"google.golang.org/api/youtube/v3"
)

type YouTubeResearcher struct {
	APIKey    string
	YtdlpPath string
	WorkDir   string
}

func New(apiKey, ytdlpPath, workDir string) *YouTubeResearcher {
	if ytdlpPath == "" {
		ytdlpPath = "yt-dlp"
	}
	if workDir == "" {
		workDir = "workspace"
	}
	return &YouTubeResearcher{
		APIKey:    apiKey,
		YtdlpPath: ytdlpPath,
		WorkDir:   workDir,
	}
}

// GenerateQueryVariations produces 3 distinct research queries for the topic.
func (r *YouTubeResearcher) GenerateQueryVariations(topic string) []string {
	topic = strings.TrimSpace(topic)
	return []string{
		fmt.Sprintf("%s explained", topic),
		fmt.Sprintf("%s truth", topic),
		fmt.Sprintf("why %s", topic),
	}
}

// Search queries YouTube for viral videos using API v3 or yt-dlp fallback.
func (r *YouTubeResearcher) Search(ctx context.Context, query string, countryCode string, maxResults int64) ([]VideoResearchItem, error) {
	if maxResults <= 0 {
		maxResults = 10
	}
	if maxResults > 25 {
		maxResults = 25
	}

	var items []VideoResearchItem
	var err error

	if r.APIKey != "" {
		items, err = r.searchWithAPI(ctx, query, countryCode, maxResults)
	}

	// If API returned no results or errored (e.g. quota exceeded or invalid key), try yt-dlp fallback
	if len(items) == 0 || err != nil {
		fallbackItems, fallbackErr := r.searchWithYtDlp(ctx, query, maxResults)
		if fallbackErr == nil && len(fallbackItems) > 0 {
			items = fallbackItems
			err = nil
		}
	}

	if err != nil && len(items) == 0 {
		return nil, fmt.Errorf("failed to search youtube videos: %w", err)
	}

	now := time.Now().UTC()
	for i := range items {
		if items[i].Velocity == 0 {
			items[i].Velocity = CalculateVelocity(items[i].ViewCount, items[i].PublishedAt, now)
		}
		if items[i].EngagementRate == 0 {
			items[i].EngagementRate = CalculateEngagementRate(items[i].ViewCount, items[i].LikeCount, items[i].CommentCount)
		}
	}

	return RankVideosByVelocity(items), nil
}

func (r *YouTubeResearcher) searchWithAPI(ctx context.Context, query string, countryCode string, maxResults int64) ([]VideoResearchItem, error) {
	service, err := youtube.NewService(ctx, option.WithAPIKey(r.APIKey))
	if err != nil {
		return nil, fmt.Errorf("youtube service init error: %w", err)
	}

	searchCall := service.Search.List([]string{"id", "snippet"}).
		Q(query).
		Type("video").
		Order("relevance").
		RelevanceLanguage("en").
		MaxResults(maxResults)

	if countryCode != "" {
		searchCall = searchCall.RegionCode(countryCode)
	}

	searchResp, err := searchCall.Do()
	if err != nil {
		return nil, fmt.Errorf("youtube search API error: %w", err)
	}

	var videoIDs []string
	idToItem := make(map[string]*VideoResearchItem)

	for _, item := range searchResp.Items {
		if item.Id == nil || item.Id.VideoId == "" {
			continue
		}
		vID := item.Id.VideoId
		videoIDs = append(videoIDs, vID)

		pubAt, _ := time.Parse(time.RFC3339, item.Snippet.PublishedAt)
		idToItem[vID] = &VideoResearchItem{
			VideoID:      vID,
			Title:        item.Snippet.Title,
			Description:  item.Snippet.Description,
			ChannelTitle: item.Snippet.ChannelTitle,
			PublishedAt:  pubAt,
		}
	}

	if len(videoIDs) == 0 {
		return nil, nil
	}

	// Fetch statistics and duration
	videoCall := service.Videos.List([]string{"statistics", "contentDetails"}).Id(videoIDs...)
	videoResp, err := videoCall.Do()
	if err != nil {
		// Return basic items if stats fetch fails
		var res []VideoResearchItem
		for _, item := range idToItem {
			res = append(res, *item)
		}
		return res, nil
	}

	for _, v := range videoResp.Items {
		if item, ok := idToItem[v.Id]; ok {
			if v.Statistics != nil {
				item.ViewCount = v.Statistics.ViewCount
				item.LikeCount = v.Statistics.LikeCount
				item.CommentCount = v.Statistics.CommentCount
			}
			if v.ContentDetails != nil {
				item.Duration = v.ContentDetails.Duration
			}
		}
	}

	var results []VideoResearchItem
	for _, id := range videoIDs {
		if item, ok := idToItem[id]; ok {
			results = append(results, *item)
		}
	}
	return results, nil
}

// searchWithYtDlp acts as a resilient fallback using yt-dlp search
func (r *YouTubeResearcher) searchWithYtDlp(ctx context.Context, query string, maxResults int64) ([]VideoResearchItem, error) {
	searchArg := fmt.Sprintf("ytsearch%d:%s", maxResults, query)
	cmd := exec.CommandContext(ctx, r.YtdlpPath, "--dump-single-json", "--flat-playlist", searchArg)
	output, err := cmd.Output()
	if err != nil {
		return nil, fmt.Errorf("yt-dlp search error: %w", err)
	}

	type YtDlpEntry struct {
		ID           string  `json:"id"`
		Title        string  `json:"title"`
		Description  string  `json:"description"`
		Uploader     string  `json:"uploader"`
		ViewCount    uint64  `json:"view_count"`
		LikeCount    uint64  `json:"like_count"`
		CommentCount uint64  `json:"comment_count"`
		Duration     float64 `json:"duration"`
	}

	type YtDlpResponse struct {
		Entries []YtDlpEntry `json:"entries"`
	}

	var resp YtDlpResponse
	if err := json.Unmarshal(output, &resp); err != nil {
		return nil, fmt.Errorf("failed to parse yt-dlp search json: %w", err)
	}

	var items []VideoResearchItem
	for _, entry := range resp.Entries {
		if entry.ID == "" {
			continue
		}
		items = append(items, VideoResearchItem{
			VideoID:      entry.ID,
			Title:        entry.Title,
			Description:  entry.Description,
			ChannelTitle: entry.Uploader,
			PublishedAt:  time.Now().Add(-24 * time.Hour), // Approximate if missing
			ViewCount:    entry.ViewCount,
			LikeCount:    entry.LikeCount,
			CommentCount: entry.CommentCount,
			Duration:     strconv.FormatFloat(entry.Duration, 'f', 0, 64) + "s",
		})
	}

	return items, nil
}

// FetchTranscript uses yt-dlp to download and parse subtitles.
func (r *YouTubeResearcher) FetchTranscript(ctx context.Context, videoID string) (string, error) {
	outDir := filepath.Join(r.WorkDir, videoID)
	if err := os.MkdirAll(outDir, 0755); err != nil {
		return "", err
	}

	outPrefix := filepath.Join(outDir, "transcript")
	videoURL := fmt.Sprintf("https://www.youtube.com/watch?v=%s", videoID)

	cmd := exec.CommandContext(ctx, r.YtdlpPath,
		"--skip-download",
		"--write-auto-sub",
		"--write-sub",
		"--sub-lang", "en,en-US,en-GB",
		"--convert-subs", "vtt",
		"-o", outPrefix,
		videoURL,
	)
	_ = cmd.Run()

	// Locate .vtt file
	files, err := os.ReadDir(outDir)
	if err != nil {
		return "", err
	}

	var vttPath string
	for _, f := range files {
		if strings.HasSuffix(f.Name(), ".vtt") {
			vttPath = filepath.Join(outDir, f.Name())
			break
		}
	}

	if vttPath == "" {
		return "", fmt.Errorf("no subtitles found for video %s", videoID)
	}

	content, err := os.ReadFile(vttPath)
	if err != nil {
		return "", err
	}

	cleaned := CleanVTT(string(content))
	return cleaned, nil
}

var (
	vttHeaderRegex = regexp.MustCompile(`(?i)^(WEBVTT|Kind:|Language:|NOTE|Style:|REGION).*`)
	vttTimeRegex   = regexp.MustCompile(`\d{2}:\d{2}(?::\d{2})?\.\d{3}\s*-->\s*\d{2}:\d{2}(?::\d{2})?\.\d{3}.*`)
	htmlTagRegex   = regexp.MustCompile(`<[^>]*>`)
	cueNumberRegex = regexp.MustCompile(`^\d+$`)
)

// CleanVTT strips WebVTT headers, timestamps, HTML tags, and duplicate text.
func CleanVTT(vtt string) string {
	lines := strings.Split(vtt, "\n")
	var cleaned []string
	var lastLine string

	for _, line := range lines {
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}
		if vttHeaderRegex.MatchString(line) {
			continue
		}
		if vttTimeRegex.MatchString(line) {
			continue
		}
		if cueNumberRegex.MatchString(line) {
			continue
		}

		line = htmlTagRegex.ReplaceAllString(line, "")
		line = strings.TrimSpace(line)
		if line == "" {
			continue
		}

		// Avoid duplicate lines that often appear in WebVTT rolling captions
		if line != lastLine {
			cleaned = append(cleaned, line)
			lastLine = line
		}
	}

	return strings.Join(cleaned, " ")
}
