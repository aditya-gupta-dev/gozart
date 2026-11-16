package pipeline

import (
	"context"
	"fmt"
	"time"

	"gozart/pkg/storage"
)

// RunDemo replays the full pipeline using the pre-made demo assets, so the UI
// can show every stage live without Ollama, TTS, Whisper or ComfyUI installed.
func (p *PipelineOrchestrator) RunDemo(ctx context.Context, jobID string) error {
	demo, err := storage.LoadDemo(storage.DemoDir())
	if err != nil {
		return err
	}
	if demo == nil {
		return fmt.Errorf("demo assets not found in %s", storage.DemoDir())
	}

	job := &storage.Job{
		ID:          jobID,
		Topic:       demo.Job.Topic,
		CountryCode: demo.Job.CountryCode,
		Format:      demo.Job.Format,
		Status:      storage.StatusRunning,
		CurrentStep: storage.StateInit,
		ProgressPct: 5,
	}
	if err := p.DB.CreateJob(job); err != nil {
		return err
	}

	wait := func(d time.Duration) bool {
		select {
		case <-ctx.Done():
			return false
		case <-time.After(d):
			return true
		}
	}

	type stage struct {
		step  string
		pct   int
		msg   string
		after func()
	}

	stages := []stage{
		{"INIT", 5, fmt.Sprintf("Demo run: replaying the pipeline for '%s' with sample data", job.Topic), nil},
		{"RESEARCHING", 15, "Phase 1: Searching viral YouTube videos and evaluating outlier velocity...", nil},
		{storage.StateResearchDone, 25, "Phase 1 complete: Analyzed 12 viral videos, avg outlier score 4.3x", nil},
		{"ANALYZING_SENTIMENT", 35, "Phase 2: Extracting viewer comments and analyzing audience sentiment...", nil},
		{storage.StateSentimentDone, 40, "Phase 2 complete: Extracted 300 comments. Primary tone: reflective", func() {
			_ = p.DB.UpdateJobSentiment(job.ID, string(demo.Sentiment))
		}},
		{"WRITING_SCRIPT", 50, "Phase 3: Synthesizing script with Scene JSON spec...", nil},
		{storage.StateScriptDone, 55, "Phase 3 complete: Script ready (6 scenes, ~628s)", func() {
			_ = p.DB.UpdateJobScript(job.ID, string(demo.Script))
		}},
		{"PARALLEL_GENERATING", 60, "Phase 4: Launching parallel tracks (Voice, Visuals, Thumbnail, SEO)...", nil},
		{"PARALLEL_GENERATING", 65, "Voice: Kokoro TTS voiceover generated, DeepFilterNet cleanup applied", nil},
		{"PARALLEL_GENERATING", 70, "Subtitles: Whisper word timestamps aligned", nil},
		{"PARALLEL_GENERATING", 75, "Thumbnail: generated and post-processed", func() {
			_ = p.DB.UpdateJobSEO(job.ID, string(demo.SEO))
		}},
		{storage.StateParallelGenDone, 80, "Phase 4 complete: All parallel tracks generated", nil},
		{"COMPOSITING", 85, "Phase 5: FFmpeg compositing (music ducking, loudness, animated subtitles)...", nil},
		{storage.StateComposited, 95, "Phase 5 complete: Final video mastered (854x480)", func() {
			_ = p.DB.UpdateJobOutputs(job.ID, demo.VideoPath, demo.ThumbnailPath)
		}},
	}

	for _, s := range stages {
		if !wait(2 * time.Second) {
			_ = p.DB.SetJobError(job.ID, "demo run cancelled")
			return ctx.Err()
		}
		if s.after != nil {
			s.after()
		}
		p.updateProgress(job, s.step, s.pct, "INFO", s.msg)
	}

	_ = p.DB.UpdateJobStatus(job.ID, storage.StatusCompleted)
	p.updateProgress(job, storage.StateComposited, 100, "INFO", "Demo pipeline finished. Ready for preview and publishing.")
	return nil
}
