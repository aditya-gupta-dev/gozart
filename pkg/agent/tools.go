package agent

import (
	"context"
	"encoding/json"
	"fmt"

	"gozart/pkg/research"

	"github.com/ollama/ollama/api"
)

// GetSearchYouTubeVideosTool returns the Ollama tool definition for YouTube search.
func GetSearchYouTubeVideosTool() api.Tool {
	props := api.NewToolPropertiesMap()
	props.Set("query", api.ToolProperty{
		Type:        api.PropertyType{"string"},
		Description: "Target search query or keywords",
	})
	props.Set("country_code", api.ToolProperty{
		Type:        api.PropertyType{"string"},
		Description: "2-letter ISO country code (e.g., US, IN, GB)",
	})
	props.Set("max_results", api.ToolProperty{
		Type:        api.PropertyType{"integer"},
		Description: "Number of videos to evaluate (5-25)",
	})

	return api.Tool{
		Type: "function",
		Function: api.ToolFunction{
			Name:        "search_youtube_videos",
			Description: "Searches for top performing viral YouTube videos for a given query, language, and country code, returning metrics and video IDs.",
			Parameters: api.ToolFunctionParameters{
				Type:       "object",
				Required:   []string{"query", "country_code"},
				Properties: props,
			},
		},
	}
}

// SearchYouTubeVideosParams holds the parsed arguments for the tool call.
type SearchYouTubeVideosParams struct {
	Query       string `json:"query"`
	CountryCode string `json:"country_code"`
	MaxResults  int64  `json:"max_results"`
}

// ExecuteSearchYouTubeVideos executes the YouTube research search given JSON arguments.
func ExecuteSearchYouTubeVideos(ctx context.Context, researcher *research.YouTubeResearcher, rawArgs string) (string, error) {
	var params SearchYouTubeVideosParams
	if err := json.Unmarshal([]byte(rawArgs), &params); err != nil {
		return "", fmt.Errorf("failed to parse tool arguments: %w", err)
	}

	if params.MaxResults <= 0 {
		params.MaxResults = 10
	}

	results, err := researcher.Search(ctx, params.Query, params.CountryCode, params.MaxResults)
	if err != nil {
		return "", fmt.Errorf("search failed: %w", err)
	}

	data, err := json.Marshal(results)
	if err != nil {
		return "", fmt.Errorf("failed to encode search results: %w", err)
	}
	return string(data), nil
}
