package handlers

import (
	"fmt"
	"io/fs"
	"net/http"
	"os"

	"gozart/pkg/config"
	"gozart/pkg/pipeline"
	"gozart/pkg/storage"
	"gozart/web"

	"github.com/go-chi/chi/v5"
	"github.com/go-chi/chi/v5/middleware"
)

type Server struct {
	Router       *chi.Mux
	Config       *config.ConfigLoader
	DB           *storage.DB
	Orchestrator *pipeline.PipelineOrchestrator
	SSEBroker    *SSEBroker
}

func NewServer(cfg *config.ConfigLoader, db *storage.DB) *Server {
	r := chi.NewRouter()
	r.Use(middleware.RequestID)
	r.Use(middleware.RealIP)
	r.Use(middleware.Logger)
	r.Use(middleware.Recoverer)

	sseBroker := NewSSEBroker()
	orch := pipeline.NewOrchestrator(db, cfg, func(jobID, level, msg string) {
		sseBroker.Broadcast(jobID, level, msg)
	})
	orch.SetProgressBroadcast(func(jobID, level, msg string, progressPct int, currentStep string) {
		sseBroker.BroadcastWithProgress(jobID, level, msg, progressPct, currentStep)
	})

	dashHandler := NewDashboardHandler(db, cfg)
	jobsHandler := NewJobsHandler(db, orch)

	s := &Server{
		Router:       r,
		Config:       cfg,
		DB:           db,
		Orchestrator: orch,
		SSEBroker:    sseBroker,
	}

	// Mount embedded static assets (/static/*)
	staticFS, err := fs.Sub(web.EmbeddedFiles, "static")
	if err == nil {
		r.Handle("/static/*", http.StripPrefix("/static/", http.FileServer(http.FS(staticFS))))
	}

	// Media preview endpoints (video & thumbnail)
	r.Get("/media/video/{id}", func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		job, err := db.GetJob(jobID)
		if err != nil || job == nil || job.VideoPath == "" {
			http.Error(w, "video not ready", http.StatusNotFound)
			return
		}
		if _, err := os.Stat(job.VideoPath); err != nil {
			http.Error(w, "video file not found", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, job.VideoPath)
	})

	r.Get("/media/thumb/{id}", func(w http.ResponseWriter, r *http.Request) {
		jobID := chi.URLParam(r, "id")
		job, err := db.GetJob(jobID)
		if err != nil || job == nil || job.ThumbnailPath == "" {
			http.Error(w, "thumbnail not ready", http.StatusNotFound)
			return
		}
		if _, err := os.Stat(job.ThumbnailPath); err != nil {
			http.Error(w, "thumbnail file not found", http.StatusNotFound)
			return
		}
		http.ServeFile(w, r, job.ThumbnailPath)
	})

	// Web Routes
	r.Get("/", dashHandler.HandleDashboard)
	r.Get("/settings", dashHandler.HandleSettings)
	r.Post("/settings/save", dashHandler.HandleSettingsSave)
	r.Get("/api/auth/youtube", dashHandler.HandleYouTubeAuth)
	r.Get("/new", jobsHandler.HandleNew)
	r.Post("/jobs/create", jobsHandler.HandleCreate)
	r.Get("/jobs/{id}", jobsHandler.HandleView)
	r.Post("/api/jobs/{id}/publish", jobsHandler.HandlePublish)
	r.Get("/api/jobs/{id}/stream", sseBroker.HandleStream)

	return s
}

func (s *Server) Start() error {
	addr := fmt.Sprintf("%s:%d", s.Config.ConfigData.Server.Host, s.Config.ConfigData.Server.Port)
	s.Config.Logger.LogFileWithStdout(fmt.Sprintf("Gozart Web UI running on http://%s", addr), 0)
	return http.ListenAndServe(addr, s.Router)
}
