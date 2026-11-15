package handlers

import (
	"context"
	"encoding/json"
	"fmt"
	"html/template"
	"net/http"
	"strconv"
	"strings"
	"time"

	"gozart/pkg/agent"
	"gozart/pkg/pipeline"
	"gozart/pkg/sentiment"
	"gozart/pkg/seo"
	"gozart/pkg/storage"
	"gozart/web"

	"github.com/go-chi/chi/v5"
)

type JobsHandler struct {
	DB           *storage.DB
	Orchestrator *pipeline.PipelineOrchestrator
}

func NewJobsHandler(db *storage.DB, orch *pipeline.PipelineOrchestrator) *JobsHandler {
	return &JobsHandler{
		DB:           db,
		Orchestrator: orch,
	}
}

func (h *JobsHandler) HandleNew(w http.ResponseWriter, r *http.Request) {
	tmpl, err := template.ParseFS(web.EmbeddedFiles, "templates/base.html", "templates/new_job.html")
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":     "Launch Autonomous Pipeline",
		"ActiveNav": "new",
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "new_job.html", data)
}

func (h *JobsHandler) HandleCreate(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "bad form data", http.StatusBadRequest)
		return
	}

	topic := r.FormValue("topic")
	if topic == "" {
		http.Error(w, "topic is required", http.StatusBadRequest)
		return
	}

	countryCode := r.FormValue("country_code")
	if countryCode == "" {
		countryCode = "US"
	}

	format := r.FormValue("format")
	if format == "" {
		format = "shorts_9_16"
	}

	tone := r.FormValue("tone")
	durSec, _ := strconv.Atoi(r.FormValue("target_duration_sec"))
	if durSec <= 0 {
		durSec = 50
	}

	modelTier := r.FormValue("model_tier")
	autoUpload := r.FormValue("auto_upload") == "true"

	jobID := fmt.Sprintf("%d", time.Now().Unix())

	params := pipeline.PipelineJobParams{
		JobID:             jobID,
		Topic:             topic,
		CountryCode:       countryCode,
		Format:            format,
		Tone:              tone,
		TargetDurationSec: durSec,
		ModelTier:         modelTier,
		AutoUpload:        autoUpload,
	}

	// Dispatch background goroutine
	go func() {
		_ = h.Orchestrator.Run(context.Background(), params)
	}()

	// Give the goroutine a brief moment to write the initial job record
	time.Sleep(100 * time.Millisecond)

	http.Redirect(w, r, "/jobs/"+jobID, http.StatusSeeOther)
}

func (h *JobsHandler) HandleView(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	job, err := h.DB.GetJob(jobID)
	if err != nil || job == nil {
		http.Error(w, "job not found", http.StatusNotFound)
		return
	}

	logs, _ := h.DB.GetJobLogs(jobID)

	var script *agent.Script
	if job.ScriptJSON != "" {
		_ = json.Unmarshal([]byte(job.ScriptJSON), &script)
	}

	var sentimentReport *sentiment.SentimentReport
	if job.SentimentJSON != "" {
		_ = json.Unmarshal([]byte(job.SentimentJSON), &sentimentReport)
	}

	var seoMeta *seo.SEOMetadata
	if job.SEOJSON != "" {
		_ = json.Unmarshal([]byte(job.SEOJSON), &seoMeta)
	}

	tmpl, err := template.ParseFS(web.EmbeddedFiles, "templates/base.html", "templates/job_view.html")
	if err != nil {
		http.Error(w, "template error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":     job.Topic,
		"ActiveNav": "dashboard",
		"Job":       job,
		"Logs":      logs,
		"Script":    script,
		"Sentiment": sentimentReport,
		"SEO":       seoMeta,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "job_view.html", data)
}

func (h *JobsHandler) HandlePublish(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	_ = r.ParseForm()
	title := r.FormValue("title")
	desc := r.FormValue("description")
	tagsRaw := r.FormValue("tags")

	var tags []string
	if tagsRaw != "" {
		for _, t := range strings.Split(tagsRaw, ",") {
			if trimmed := strings.TrimSpace(t); trimmed != "" {
				tags = append(tags, trimmed)
			}
		}
	}

	go func() {
		_ = h.Orchestrator.PublishToYouTubeWithMetadata(jobID, title, desc, tags)
	}()

	http.Redirect(w, r, "/jobs/"+jobID, http.StatusSeeOther)
}
