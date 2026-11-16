package storage

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
)

const DemoTopic = "5 Unspoken Rules That Reveal Your True Character"

type DemoLog struct {
	Level   string `json:"level"`
	Message string `json:"message"`
}

// DemoData is the sample job stored in <dir>/demo_job.json.
type DemoData struct {
	Job       Job             `json:"job"`
	Script    json.RawMessage `json:"script"`
	Sentiment json.RawMessage `json:"sentiment"`
	SEO       json.RawMessage `json:"seo"`
	Logs      []DemoLog       `json:"logs"`

	VideoPath     string `json:"-"`
	ThumbnailPath string `json:"-"`
}

// DemoDir returns the folder holding the demo assets (GOZART_DEMO_DIR or "demo").
func DemoDir() string {
	if v := os.Getenv("GOZART_DEMO_DIR"); v != "" {
		return v
	}
	return "demo"
}

// LoadDemo reads demo_job.json from dir. It returns nil, nil if the file is missing.
func LoadDemo(dir string) (*DemoData, error) {
	data, err := os.ReadFile(filepath.Join(dir, "demo_job.json"))
	if err != nil {
		return nil, nil
	}

	var demo DemoData
	if err := json.Unmarshal(data, &demo); err != nil {
		return nil, fmt.Errorf("invalid demo_job.json: %w", err)
	}
	demo.VideoPath = filepath.Join(dir, "demo.mp4")
	demo.ThumbnailPath = filepath.Join(dir, "thumbnail.jpg")
	return &demo, nil
}

// SeedDemo inserts the finished sample job so the dashboard has something to show.
// It does nothing if the demo folder is missing or the job already exists.
func (d *DB) SeedDemo(dir string) error {
	demo, err := LoadDemo(dir)
	if err != nil || demo == nil {
		return err
	}

	if existing, _ := d.GetJob(demo.Job.ID); existing != nil {
		return nil
	}

	job := demo.Job
	job.ScriptJSON = string(demo.Script)
	job.SentimentJSON = string(demo.Sentiment)
	job.SEOJSON = string(demo.SEO)
	job.VideoPath = demo.VideoPath
	job.ThumbnailPath = demo.ThumbnailPath

	if err := d.CreateJob(&job); err != nil {
		return err
	}
	for _, l := range demo.Logs {
		_ = d.AddJobLog(job.ID, l.Level, l.Message)
	}
	return nil
}
