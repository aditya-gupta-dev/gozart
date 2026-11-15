package research

import (
	"sort"
	"time"
)

type VideoResearchItem struct {
	VideoID        string    `json:"video_id"`
	Title          string    `json:"title"`
	Description    string    `json:"description"`
	ChannelTitle   string    `json:"channel_title"`
	PublishedAt    time.Time `json:"published_at"`
	ViewCount      uint64    `json:"view_count"`
	LikeCount      uint64    `json:"like_count"`
	CommentCount   uint64    `json:"comment_count"`
	Duration       string    `json:"duration"`
	Velocity       float64   `json:"velocity"`
	EngagementRate float64   `json:"engagement_rate"`
	Transcript     string    `json:"transcript,omitempty"`
}

// CalculateVelocity computes: ViewCount / (Hours Since Published + 2)
func CalculateVelocity(viewCount uint64, publishedAt time.Time, now time.Time) float64 {
	hours := now.Sub(publishedAt).Hours()
	if hours < 0 {
		hours = 0
	}
	return float64(viewCount) / (hours + 2.0)
}

// CalculateEngagementRate computes: ((LikeCount + CommentCount) / ViewCount) * 100
func CalculateEngagementRate(viewCount, likeCount, commentCount uint64) float64 {
	if viewCount == 0 {
		return 0.0
	}
	return (float64(likeCount+commentCount) / float64(viewCount)) * 100.0
}

// RankVideosByVelocity sorts the videos by velocity descending.
func RankVideosByVelocity(items []VideoResearchItem) []VideoResearchItem {
	sorted := make([]VideoResearchItem, len(items))
	copy(sorted, items)
	sort.Slice(sorted, func(i, j int) bool {
		return sorted[i].Velocity > sorted[j].Velocity
	})
	return sorted
}
