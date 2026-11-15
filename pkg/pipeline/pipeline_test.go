package pipeline

import (
	"context"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"gozart/pkg/config"
	"gozart/pkg/logger"
	"gozart/pkg/storage"
)

func TestPipelineEndToEnd(t *testing.T) {
	tempRoot := t.TempDir()
	dbPath := filepath.Join(tempRoot, "test.db")
	workDir := filepath.Join(tempRoot, "workspace")
	outDir := filepath.Join(tempRoot, "output")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	l := logger.New()
	defer l.Close()

	cfg := &config.ConfigLoader{
		Pwd:    tempRoot,
		Logger: l,
	}
	cfg.ConfigData.Storage.DBPath = dbPath
	cfg.ConfigData.Storage.WorkDir = workDir
	cfg.ConfigData.Storage.OutputDir = outDir
	cfg.ConfigData.FfmpegPath = "ffmpeg"
	cfg.ConfigData.YtdlpPath = "yt-dlp"

	// Pre-create job checkpointing from StateSentimentDone to test full parallel gen & composition deterministically
	jobID := "test-pipeline-123"
	initialJob := &storage.Job{
		ID:            jobID,
		Topic:         "Why NASA Left The Moon",
		CountryCode:   "US",
		Format:        "shorts_9_16",
		Status:        storage.StatusQueued,
		CurrentStep:   storage.StateSentimentDone,
		SentimentJSON: `{"audience_sentiment":{"positive_triggers":["data"],"negative_friction":["pacing"],"unanswered_questions":["why stop?"],"emotional_tone":"curiosity"}}`,
	}
	if err := db.CreateJob(initialJob); err != nil {
		t.Fatalf("failed to create initial job: %v", err)
	}

	var logs []string
	orch := NewOrchestrator(db, cfg, func(jobID, level, msg string) {
		logs = append(logs, msg)
	})

	params := PipelineJobParams{
		JobID:             jobID,
		Topic:             "Why NASA Left The Moon",
		CountryCode:       "US",
		Format:            "shorts_9_16",
		Tone:              "sensational",
		TargetDurationSec: 3,
		AutoUpload:        false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := orch.Run(ctx, params); err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	// Verify job in database
	job, err := db.GetJob(jobID)
	if err != nil || job == nil {
		t.Fatalf("failed to retrieve job from DB: %v", err)
	}

	if job.Status != storage.StatusCompleted {
		t.Errorf("expected status %s, got %s (error: %s)", storage.StatusCompleted, job.Status, job.ErrorMessage)
	}

	if job.ProgressPct != 100 {
		t.Errorf("expected progress 100, got %d", job.ProgressPct)
	}

	// Verify generated files
	if job.VideoPath == "" {
		t.Errorf("expected non-empty video path")
	} else if _, err := os.Stat(job.VideoPath); err != nil {
		t.Errorf("rendered video file does not exist: %v", err)
	}

	if job.ThumbnailPath == "" {
		t.Errorf("expected non-empty thumbnail path")
	} else if _, err := os.Stat(job.ThumbnailPath); err != nil {
		t.Errorf("thumbnail file does not exist: %v", err)
	}

	if len(logs) == 0 {
		t.Errorf("expected logs to be broadcasted during pipeline")
	}
}

func TestPipelineResumeFromResearchDone(t *testing.T) {
	tempRoot := t.TempDir()
	dbPath := filepath.Join(tempRoot, "test.db")
	workDir := filepath.Join(tempRoot, "workspace")
	outDir := filepath.Join(tempRoot, "output")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	l := logger.New()
	defer l.Close()

	cfg := &config.ConfigLoader{
		Pwd:    tempRoot,
		Logger: l,
	}
	cfg.ConfigData.Storage.DBPath = dbPath
	cfg.ConfigData.Storage.WorkDir = workDir
	cfg.ConfigData.Storage.OutputDir = outDir
	cfg.ConfigData.FfmpegPath = "ffmpeg"
	cfg.ConfigData.YtdlpPath = "/bin/false"

	jobID := "test-resume-research"
	jobDir := filepath.Join(workDir, jobID)
	if err := os.MkdirAll(jobDir, 0755); err != nil {
		t.Fatalf("failed to create jobDir: %v", err)
	}
	if err := os.WriteFile(filepath.Join(jobDir, "transcripts.txt"), []byte("Competitor transcript content"), 0644); err != nil {
		t.Fatalf("failed to write transcripts.txt: %v", err)
	}

	initialJob := &storage.Job{
		ID:          jobID,
		Topic:       "Quantum Computing Explained",
		CountryCode: "US",
		Format:      "shorts_9_16",
		Status:      storage.StatusQueued,
		CurrentStep: storage.StateResearchDone,
		ProgressPct: 25,
	}
	if err := db.CreateJob(initialJob); err != nil {
		t.Fatalf("failed to create initial job: %v", err)
	}

	var logMsgs []string
	orch := NewOrchestrator(db, cfg, func(jobID, level, msg string) {
		logMsgs = append(logMsgs, msg)
	})

	params := PipelineJobParams{
		JobID:             jobID,
		Topic:             "Quantum Computing Explained",
		CountryCode:       "US",
		Format:            "shorts_9_16",
		Tone:              "educational",
		TargetDurationSec: 3,
		AutoUpload:        false,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	if err := orch.Run(ctx, params); err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	job, err := db.GetJob(jobID)
	if err != nil || job == nil {
		t.Fatalf("failed to retrieve job: %v", err)
	}

	if job.Status != storage.StatusCompleted {
		t.Errorf("expected status %s, got %s", storage.StatusCompleted, job.Status)
	}
	if job.CurrentStep != storage.StateComposited {
		t.Errorf("expected step %s, got %s", storage.StateComposited, job.CurrentStep)
	}
	if job.ProgressPct != 100 {
		t.Errorf("expected progress 100, got %d", job.ProgressPct)
	}
}

func TestPipelineResumeFromParallelGenDone(t *testing.T) {
	tempRoot := t.TempDir()
	dbPath := filepath.Join(tempRoot, "test.db")
	workDir := filepath.Join(tempRoot, "workspace")
	outDir := filepath.Join(tempRoot, "output")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	l := logger.New()
	defer l.Close()

	cfg := &config.ConfigLoader{
		Pwd:    tempRoot,
		Logger: l,
	}
	cfg.ConfigData.Storage.DBPath = dbPath
	cfg.ConfigData.Storage.WorkDir = workDir
	cfg.ConfigData.Storage.OutputDir = outDir
	cfg.ConfigData.FfmpegPath = "ffmpeg"
	cfg.ConfigData.YtdlpPath = "yt-dlp"

	jobID := "test-resume-parallel"
	jobDir := filepath.Join(workDir, jobID)
	jobOutDir := filepath.Join(outDir, jobID)
	_ = os.MkdirAll(jobDir, 0755)
	_ = os.MkdirAll(jobOutDir, 0755)

	// Generate valid test audio and video for compositing
	rawVoice := filepath.Join(jobDir, "raw_voice.wav")
	enhancedVoice := filepath.Join(jobDir, "enhanced_voice.wav")
	visuals := filepath.Join(jobDir, "visuals.mp4")
	thumb := filepath.Join(jobOutDir, "thumbnail.jpg")

	_ = exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "1", rawVoice).Run()
	_ = exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "anullsrc=r=48000:cl=stereo", "-t", "1", enhancedVoice).Run()
	_ = exec.Command("ffmpeg", "-y", "-f", "lavfi", "-i", "testsrc=duration=1:size=720x1280:rate=30", "-pix_fmt", "yuv420p", visuals).Run()
	_ = os.WriteFile(thumb, []byte("fake-jpeg-data"), 0644)

	initialJob := &storage.Job{
		ID:          jobID,
		Topic:       "Black Holes",
		CountryCode: "US",
		Format:      "shorts_9_16",
		Status:      storage.StatusQueued,
		CurrentStep: storage.StateParallelGenDone,
		ProgressPct: 80,
	}
	if err := db.CreateJob(initialJob); err != nil {
		t.Fatalf("failed to create initial job: %v", err)
	}

	var logMsgs []string
	orch := NewOrchestrator(db, cfg, func(jobID, level, msg string) {
		logMsgs = append(logMsgs, msg)
	})

	params := PipelineJobParams{
		JobID:             jobID,
		Topic:             "Black Holes",
		CountryCode:       "US",
		Format:            "shorts_9_16",
		TargetDurationSec: 1,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()

	if err := orch.Run(ctx, params); err != nil {
		t.Fatalf("pipeline run failed: %v", err)
	}

	// Verify Phase 4 was skipped
	foundReuseLog := false
	for _, m := range logMsgs {
		if strings.Contains(m, "reusing generated media assets") {
			foundReuseLog = true
			break
		}
	}
	if !foundReuseLog {
		t.Errorf("expected log stating Phase 4 was skipped and assets reused")
	}

	job, err := db.GetJob(jobID)
	if err != nil || job == nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if job.Status != storage.StatusCompleted {
		t.Errorf("expected status %s, got %s", storage.StatusCompleted, job.Status)
	}
	if job.ProgressPct != 100 {
		t.Errorf("expected progress 100, got %d", job.ProgressPct)
	}
}

func TestPipelineParallelGenCriticalFailure(t *testing.T) {
	tempRoot := t.TempDir()
	dbPath := filepath.Join(tempRoot, "test.db")
	workDir := filepath.Join(tempRoot, "workspace")
	outDir := filepath.Join(tempRoot, "output")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	l := logger.New()
	defer l.Close()

	cfg := &config.ConfigLoader{
		Pwd:    tempRoot,
		Logger: l,
	}
	cfg.ConfigData.Storage.DBPath = dbPath
	cfg.ConfigData.Storage.WorkDir = workDir
	cfg.ConfigData.Storage.OutputDir = outDir
	cfg.ConfigData.FfmpegPath = "ffmpeg"
	cfg.ConfigData.YtdlpPath = "yt-dlp"

	jobID := "test-fail-parallel"
	initialJob := &storage.Job{
		ID:          jobID,
		Topic:       "Failing Pipeline",
		CountryCode: "US",
		Format:      "shorts_9_16",
		Status:      storage.StatusQueued,
		CurrentStep: storage.StateScriptDone,
		ScriptJSON:  `{"title_idea":"Fail Test","scenes":[{"scene_index":1,"voiceover":"test","duration_sec":2}]}`,
		ProgressPct: 55,
	}
	if err := db.CreateJob(initialJob); err != nil {
		t.Fatalf("failed to create initial job: %v", err)
	}

	orch := NewOrchestrator(db, cfg, nil)
	// Break remotion ffmpeg binary so Track 5B fails fatally
	orch.Remotion.FfmpegBinary = "/nonexistent/invalid/ffmpeg"

	params := PipelineJobParams{
		JobID:             jobID,
		Topic:             "Failing Pipeline",
		CountryCode:       "US",
		Format:            "shorts_9_16",
		TargetDurationSec: 2,
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	err = orch.Run(ctx, params)
	if err == nil {
		t.Fatalf("expected pipeline run to return error when parallel track fails, got nil")
	}

	job, getErr := db.GetJob(jobID)
	if getErr != nil || job == nil {
		t.Fatalf("failed to retrieve job: %v", getErr)
	}

	if job.Status != storage.StatusFailed {
		t.Errorf("expected job status %s, got %s", storage.StatusFailed, job.Status)
	}
	if job.ErrorMessage == "" {
		t.Errorf("expected non-empty ErrorMessage on failed job")
	}

	// Verify final render was NOT created
	finalPath := filepath.Join(outDir, jobID, "final_render.mp4")
	if _, err := os.Stat(finalPath); err == nil {
		t.Errorf("final_render.mp4 should not exist when pipeline fails in parallel gen")
	}
}

func TestPipelinePublishToYouTubeErrors(t *testing.T) {
	tempRoot := t.TempDir()
	dbPath := filepath.Join(tempRoot, "test.db")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}
	defer db.Close()

	l := logger.New()
	defer l.Close()

	cfg := &config.ConfigLoader{
		Pwd:    tempRoot,
		Logger: l,
	}

	orch := NewOrchestrator(db, cfg, nil)

	// Non-existent job
	if err := orch.PublishToYouTubeWithMetadata("non-existent-id", "Title", "Desc", nil); err == nil {
		t.Errorf("expected error publishing non-existent job")
	}

	// Job with missing video file
	job := &storage.Job{
		ID:        "job-missing-video",
		Topic:     "Missing Video",
		Status:    storage.StatusCompleted,
		VideoPath: filepath.Join(tempRoot, "nonexistent.mp4"),
	}
	_ = db.CreateJob(job)

	if err := orch.PublishToYouTubeWithMetadata(job.ID, "Title", "Desc", nil); err == nil {
		t.Errorf("expected error publishing job with missing video file")
	}
}
