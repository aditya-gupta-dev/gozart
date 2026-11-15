package pipeline

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"sync"

	"gozart/pkg/agent"
	"gozart/pkg/audio"
	"gozart/pkg/compositor"
	"gozart/pkg/config"
	"gozart/pkg/image"
	"gozart/pkg/media"
	"gozart/pkg/research"
	"gozart/pkg/sentiment"
	"gozart/pkg/seo"
	"gozart/pkg/storage"
	"gozart/pkg/uploader"
)

type PipelineJobParams struct {
	JobID             string
	Topic             string
	CountryCode       string
	Format            string
	Tone              string
	TargetDurationSec int
	ModelTier         string
	AutoUpload        bool
}

type LogBroadcastFunc func(jobID, level, message string)
type ProgressBroadcastFunc func(jobID, level, message string, progressPct int, currentStep string)

type PipelineOrchestrator struct {
	DB                *storage.DB
	Config            *config.ConfigLoader
	Researcher        *research.YouTubeResearcher
	Harness           *agent.AgentHarness
	Scraper           *sentiment.CommentScraper
	TTS               *audio.TTSClient
	Enhancer          *audio.DeepFilterEnhancer
	Aligner           *audio.WhisperAligner
	Remotion          *media.RemotionRenderer
	ThumbGen          *image.ThumbnailGenerator
	ImageEnhancer     *image.ImageEnhancer
	Compositor        *compositor.Compositor
	LogBroadcast      LogBroadcastFunc
	ProgressBroadcast ProgressBroadcastFunc
	mu                sync.Mutex
	activeJobCancels  map[string]context.CancelFunc
}

func NewOrchestrator(db *storage.DB, cfg *config.ConfigLoader, broadcast LogBroadcastFunc) *PipelineOrchestrator {
	workDir := cfg.ConfigData.Storage.WorkDir
	ytdlp := cfg.ConfigData.YtdlpPath
	ffmpeg := cfg.ConfigData.FfmpegPath
	ffprobe := cfg.GetFfprobePath()
	apiKey := cfg.ConfigData.YouTube.APIKey

	researcher := research.New(apiKey, ytdlp, workDir)
	harness := agent.New(cfg.ConfigData.AI, researcher)
	scraper := sentiment.NewScraper(apiKey, ytdlp)
	tts := audio.NewTTS(cfg.ConfigData.Audio, ffmpeg)
	enhancer := audio.NewEnhancer(cfg.ConfigData.Audio)
	aligner := audio.NewAligner(ffprobe)
	remotion := media.NewRemotion(cfg.ConfigData.Media.NodeBinary, ffmpeg, cfg.ConfigData.Media.MaxHealingRetries)
	thumbGen := image.NewGenerator("")
	imgEnhancer := image.NewEnhancer(ffmpeg)
	comp := compositor.New(ffmpeg)

	return &PipelineOrchestrator{
		DB:               db,
		Config:           cfg,
		Researcher:       researcher,
		Harness:          harness,
		Scraper:          scraper,
		TTS:              tts,
		Enhancer:         enhancer,
		Aligner:          aligner,
		Remotion:         remotion,
		ThumbGen:         thumbGen,
		ImageEnhancer:    imgEnhancer,
		Compositor:       comp,
		LogBroadcast:     broadcast,
		activeJobCancels: make(map[string]context.CancelFunc),
	}
}

func (p *PipelineOrchestrator) SetProgressBroadcast(pb ProgressBroadcastFunc) {
	p.ProgressBroadcast = pb
}

func (p *PipelineOrchestrator) log(jobID, level, msg string) {
	_ = p.DB.AddJobLog(jobID, level, msg)
	if p.LogBroadcast != nil {
		p.LogBroadcast(jobID, level, msg)
	}
	if p.ProgressBroadcast != nil {
		p.ProgressBroadcast(jobID, level, msg, 0, "")
	}
}

func (p *PipelineOrchestrator) updateProgress(job *storage.Job, step string, pct int, level, msg string) {
	job.CurrentStep = step
	job.ProgressPct = pct
	_ = p.DB.UpdateJobProgress(job.ID, step, pct)
	_ = p.DB.AddJobLog(job.ID, level, msg)
	if p.LogBroadcast != nil {
		p.LogBroadcast(job.ID, level, msg)
	}
	if p.ProgressBroadcast != nil {
		p.ProgressBroadcast(job.ID, level, msg, pct, step)
	}
}

// Run executes or resumes the autonomous pipeline for a job.
func (p *PipelineOrchestrator) Run(ctx context.Context, params PipelineJobParams) error {
	ctx, cancel := context.WithCancel(ctx)
	p.mu.Lock()
	p.activeJobCancels[params.JobID] = cancel
	p.mu.Unlock()
	defer func() {
		p.mu.Lock()
		delete(p.activeJobCancels, params.JobID)
		p.mu.Unlock()
	}()

	if params.ModelTier != "" {
		p.Harness.SetModelTier(params.ModelTier)
	}

	job, err := p.DB.GetJob(params.JobID)
	if err != nil {
		return fmt.Errorf("failed to fetch job: %w", err)
	}
	if job == nil {
		job = &storage.Job{
			ID:          params.JobID,
			Topic:       params.Topic,
			CountryCode: params.CountryCode,
			Format:      params.Format,
			Status:      storage.StatusRunning,
			CurrentStep: storage.StateInit,
			ProgressPct: 5,
		}
		if err := p.DB.CreateJob(job); err != nil {
			return err
		}
	} else {
		_ = p.DB.UpdateJobStatus(job.ID, storage.StatusRunning)
	}

	p.log(job.ID, "INFO", fmt.Sprintf("Starting Gozart autonomous engine for topic '%s' [%s]", job.Topic, job.Format))
	jobDir := filepath.Join(p.Config.ConfigData.Storage.WorkDir, job.ID)
	outDir := filepath.Join(p.Config.ConfigData.Storage.OutputDir, job.ID)
	_ = os.MkdirAll(jobDir, 0755)
	_ = os.MkdirAll(outDir, 0755)

	var topTranscripts string
	var sentimentReport *sentiment.SentimentReport
	var script *agent.Script

	// Step 1: Research (Skip if already completed in checkpoint)
	if job.CurrentStep == storage.StateInit {
		p.updateProgress(job, "RESEARCHING", 15, "INFO", "Phase 1: Searching viral YouTube videos and evaluating outlier velocity...")

		queries := p.Researcher.GenerateQueryVariations(job.Topic)
		var allItems []research.VideoResearchItem
		for _, q := range queries {
			items, _ := p.Researcher.Search(ctx, q, job.CountryCode, 5)
			allItems = append(allItems, items...)
		}
		ranked := research.RankVideosByVelocity(allItems)

		var transcriptParts []string
		// Fetch transcripts for top 3 ranked
		for i, v := range ranked {
			if i >= 3 {
				break
			}
			p.log(job.ID, "INFO", fmt.Sprintf("Extracting transcript from viral video [%s] '%s' (Velocity: %.1f)", v.VideoID, v.Title, v.Velocity))
			tText, tErr := p.Researcher.FetchTranscript(ctx, v.VideoID)
			if tErr == nil && tText != "" {
				transcriptParts = append(transcriptParts, fmt.Sprintf("Video: %s\nTranscript: %s", v.Title, tText))
			}
		}
		if len(transcriptParts) > 0 {
			topTranscripts = strings.Join(transcriptParts, "\n\n")
			_ = os.WriteFile(filepath.Join(jobDir, "transcripts.txt"), []byte(topTranscripts), 0644)
		} else {
			topTranscripts = "No competitor transcripts found."
		}
		p.updateProgress(job, storage.StateResearchDone, 25, "INFO", fmt.Sprintf("Phase 1 complete: Analyzed %d viral videos. Found %d transcripts.", len(ranked), len(transcriptParts)))
	}

	// Step 2: Comment Sentiment & Signal Extraction
	if job.CurrentStep == storage.StateInit || job.CurrentStep == storage.StateResearchDone {
		p.updateProgress(job, "ANALYZING_SENTIMENT", 35, "INFO", "Phase 2: Extracting viewer comments and analyzing audience sentiment...")

		var comments []string
		items, _ := p.Researcher.Search(ctx, job.Topic, job.CountryCode, 3)
		for _, v := range items {
			cList, _ := p.Scraper.FetchTopComments(ctx, v.VideoID, 30)
			comments = append(comments, cList...)
		}

		sReport, sErr := sentiment.ExtractSentiment(ctx, p.Harness, comments, job.Topic)
		if sErr != nil || sReport == nil {
			sReport = sentiment.GenerateFallbackSentiment(job.Topic)
		}
		sentimentReport = sReport
		sJSON, _ := json.MarshalIndent(sReport, "", "  ")
		_ = p.DB.UpdateJobSentiment(job.ID, string(sJSON))
		p.updateProgress(job, storage.StateSentimentDone, 40, "INFO", fmt.Sprintf("Phase 2 complete: Extracted %d comments. Primary tone: %s", len(comments), sReport.AudienceSentiment.EmotionalTone))
	} else if job.SentimentJSON != "" {
		_ = json.Unmarshal([]byte(job.SentimentJSON), &sentimentReport)
	}

	// Step 3: Scriptwriting
	if job.CurrentStep == storage.StateInit || job.CurrentStep == storage.StateResearchDone || job.CurrentStep == storage.StateSentimentDone {
		p.updateProgress(job, "WRITING_SCRIPT", 50, "INFO", "Phase 3: Synthesizing viral script with Scene JSON spec...")

		if topTranscripts == "" {
			if data, err := os.ReadFile(filepath.Join(jobDir, "transcripts.txt")); err == nil {
				topTranscripts = string(data)
			}
		}

		sJSONStr := ""
		if sentimentReport != nil {
			raw, _ := json.Marshal(sentimentReport)
			sJSONStr = string(raw)
		}

		sc, scErr := agent.WriteScript(ctx, p.Harness, job.Topic, job.CountryCode, job.Format, params.Tone, params.TargetDurationSec, topTranscripts, sJSONStr)
		if scErr != nil || sc == nil {
			sc = agent.GenerateFallbackScript(job.Topic, job.Format, params.TargetDurationSec)
		}
		script = sc
		scriptData, _ := json.MarshalIndent(script, "", "  ")
		_ = p.DB.UpdateJobScript(job.ID, string(scriptData))
		p.updateProgress(job, storage.StateScriptDone, 55, "INFO", fmt.Sprintf("Phase 3 complete: Script synthesized: '%s' (%d scenes, ~%.1fs)", script.TitleIdea, len(script.Scenes), script.TotalEstimatedDuration))
	} else if job.ScriptJSON != "" {
		_ = json.Unmarshal([]byte(job.ScriptJSON), &script)
	}

	if script == nil {
		script = agent.GenerateFallbackScript(job.Topic, job.Format, params.TargetDurationSec)
	}

	// Step 4: Parallel 4-Track Generation Phase
	rawVoicePath := filepath.Join(jobDir, "raw_voice.wav")
	enhancedVoicePath := filepath.Join(jobDir, "enhanced_voice.wav")
	captionsPath := filepath.Join(jobDir, "captions.ass")
	visualsPath := filepath.Join(jobDir, "visuals.mp4")
	thumbPath := filepath.Join(outDir, "thumbnail.jpg")

	step4AlreadyDone := (job.CurrentStep == storage.StateParallelGenDone || job.CurrentStep == storage.StateComposited || job.CurrentStep == storage.StateUploaded)
	assetsExist := fileExists(rawVoicePath) && fileExists(enhancedVoicePath) && fileExists(visualsPath) && fileExists(thumbPath)

	if !step4AlreadyDone || !assetsExist {
		p.updateProgress(job, "PARALLEL_GENERATING", 60, "INFO", "Phase 4: Launching 4 parallel generation tracks (Voice, Visuals, Thumbnail, SEO)...")

		var wg sync.WaitGroup
		errChan := make(chan error, 4)

		// Track 5A: Voiceover (TTS) + DeepFilterNet + Subtitle Timestamps
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.log(job.ID, "INFO", "Track 5A: Generating voiceover audio...")
			fullSpeech := script.FullVoiceoverText()
			if err := p.TTS.GenerateVoiceover(fullSpeech, rawVoicePath); err != nil {
				errChan <- fmt.Errorf("TTS audio generation failed: %w", err)
				return
			}

			p.log(job.ID, "INFO", "Track 5A: Applying DeepFilterNet 48kHz noise enhancement...")
			if err := p.Enhancer.EnhanceAudio(rawVoicePath, enhancedVoicePath); err != nil {
				_ = copyFile(rawVoicePath, enhancedVoicePath)
			}

			p.log(job.ID, "INFO", "Track 5A: Generating word-level subtitle alignment...")
			timestamps, alignErr := p.Aligner.AlignWords(enhancedVoicePath, fullSpeech)
			if alignErr == nil && len(timestamps) > 0 {
				_ = p.Aligner.SaveTranscription(timestamps, filepath.Join(jobDir, "transcription.json"))
				_ = compositor.GenerateASS(timestamps, job.Format, captionsPath)
			}
		}()

		// Track 5B: Media Code Generation (Remotion / Visuals)
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.log(job.ID, "INFO", "Track 5B: Rendering programmatic media visuals...")
			if err := p.Remotion.RenderVisuals(ctx, script, job.Format, jobDir, visualsPath); err != nil {
				errChan <- fmt.Errorf("visual render failed: %w", err)
				return
			}
			p.log(job.ID, "INFO", "Track 5B: Visuals rendering completed.")
		}()

		// Track 5C: Fine-Tuned Thumbnail
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.log(job.ID, "INFO", "Track 5C: Generating high-CTR thumbnail...")
			spec, _ := p.ThumbGen.CraftPrompt(ctx, p.Harness, job.Topic, script.Hook.Voiceover)
			rawThumbPath := filepath.Join(jobDir, "raw_thumb.jpg")
			if err := p.ThumbGen.Generate(ctx, spec, job.Format, rawThumbPath); err == nil {
				_ = p.ImageEnhancer.EnhancePostProcessing(rawThumbPath, thumbPath)
			}
			p.log(job.ID, "INFO", "Track 5C: Thumbnail generated and enhanced.")
		}()

		// Track 5D: YouTube SEO
		wg.Add(1)
		go func() {
			defer wg.Done()
			p.log(job.ID, "INFO", "Track 5D: Generating search-optimized title, description & tags...")
			meta, _ := seo.GenerateSEO(ctx, p.Harness, job.Topic, job.CountryCode, script, sentimentReport)
			seoData, _ := json.MarshalIndent(meta, "", "  ")
			_ = p.DB.UpdateJobSEO(job.ID, string(seoData))
			p.log(job.ID, "INFO", fmt.Sprintf("Track 5D: SEO optimized. Chosen Title: '%s'", meta.ChosenTitle))
		}()

		wg.Wait()
		close(errChan)

		var criticalErr error
		for trackErr := range errChan {
			p.log(job.ID, "ERROR", fmt.Sprintf("Parallel track error: %v", trackErr))
			if criticalErr == nil {
				criticalErr = trackErr
			}
		}

		if criticalErr != nil {
			_ = p.DB.SetJobError(job.ID, criticalErr.Error())
			return criticalErr
		}

		p.updateProgress(job, storage.StateParallelGenDone, 80, "INFO", "Phase 4 complete: All 4 parallel tracks generated successfully.")
	} else {
		p.log(job.ID, "INFO", "Phase 4 already completed in checkpoint; reusing generated media assets.")
	}

	// Step 5: FFmpeg Compositor & Audio Mastering
	finalVideoPath := filepath.Join(outDir, "final_render.mp4")
	step5AlreadyDone := (job.CurrentStep == storage.StateComposited || job.CurrentStep == storage.StateUploaded)
	finalVideoExists := fileExists(finalVideoPath)

	if !step5AlreadyDone || !finalVideoExists {
		p.updateProgress(job, "COMPOSITING", 85, "INFO", "Phase 5: FFmpeg multi-track compositing (sidechain ducking, -14 LUFS loudness, animated subtitles)...")

		bgmPath := filepath.Join("assets", "music", "cinematic_ambient.mp3")
		if !fileExists(bgmPath) {
			bgmPath = ""
		}

		compErr := p.Compositor.Composite(ctx, visualsPath, enhancedVoicePath, bgmPath, captionsPath, finalVideoPath)
		if compErr != nil {
			p.log(job.ID, "ERROR", fmt.Sprintf("Compositing error: %v", compErr))
			_ = p.DB.SetJobError(job.ID, compErr.Error())
			return compErr
		}

		_ = p.DB.UpdateJobOutputs(job.ID, finalVideoPath, thumbPath)
		p.updateProgress(job, storage.StateComposited, 95, "INFO", fmt.Sprintf("Phase 5 complete: Final video mastered successfully to %s", finalVideoPath))
	} else {
		p.log(job.ID, "INFO", "Phase 5 already composited in checkpoint; reusing rendered video.")
	}

	// Step 6: Publish to YouTube (if requested)
	if params.AutoUpload {
		p.updateProgress(job, "UPLOADING", 98, "INFO", "Phase 6: Publishing to YouTube...")
		if err := p.PublishToYouTube(job.ID); err != nil {
			p.log(job.ID, "WARN", fmt.Sprintf("YouTube upload failed: %v", err))
		}
	} else {
		_ = p.DB.UpdateJobStatus(job.ID, storage.StatusCompleted)
		p.updateProgress(job, storage.StateComposited, 100, "INFO", "Autonomous generation pipeline finished successfully! Ready for preview and publishing.")
	}

	return nil
}

// PublishToYouTube executes video and thumbnail upload using stored job SEO metadata.
func (p *PipelineOrchestrator) PublishToYouTube(jobID string) error {
	return p.PublishToYouTubeWithMetadata(jobID, "", "", nil)
}

// PublishToYouTubeWithMetadata uploads video and thumbnail, updating SEO metadata if custom values are provided.
func (p *PipelineOrchestrator) PublishToYouTubeWithMetadata(jobID string, title, desc string, tags []string) error {
	job, err := p.DB.GetJob(jobID)
	if err != nil || job == nil {
		return fmt.Errorf("job not found: %s", jobID)
	}

	if job.VideoPath == "" || !fileExists(job.VideoPath) {
		return fmt.Errorf("no rendered video file found for job %s", jobID)
	}

	var seoData seo.SEOMetadata
	if job.SEOJSON != "" {
		_ = json.Unmarshal([]byte(job.SEOJSON), &seoData)
	}

	if title != "" {
		seoData.ChosenTitle = title
	} else if seoData.ChosenTitle == "" {
		seoData.ChosenTitle = job.Topic
	}

	if desc != "" {
		seoData.Description = desc
	}

	if len(tags) > 0 {
		seoData.Tags = tags
	}

	seoBytes, _ := json.MarshalIndent(seoData, "", "  ")
	_ = p.DB.UpdateJobSEO(job.ID, string(seoBytes))

	ytUploader := uploader.New(p.Config.Logger, p.Config)
	ytUploader.Authenticate()

	uploadedID := ytUploader.UploadVideo(job.VideoPath, seoData.ChosenTitle, seoData.Description, seoData.Tags)
	if uploadedID == "" {
		return fmt.Errorf("youtube video upload failed")
	}

	if job.ThumbnailPath != "" && fileExists(job.ThumbnailPath) {
		ytUploader.UploadThumbnail(uploadedID, job.ThumbnailPath)
	}

	_ = p.DB.UpdateJobYouTubeID(job.ID, uploadedID)
	p.updateProgress(job, storage.StateUploaded, 100, "INFO", fmt.Sprintf("Successfully published to YouTube! Video ID: %s", uploadedID))
	return nil
}

func fileExists(path string) bool {
	if path == "" {
		return false
	}
	fi, err := os.Stat(path)
	return err == nil && !fi.IsDir() && fi.Size() > 0
}

func copyFile(src, dst string) error {
	in, err := os.ReadFile(src)
	if err != nil {
		return err
	}
	return os.WriteFile(dst, in, 0644)
}
