package handlers

import (
	"net/http"
	"net/http/httptest"
	"net/url"
	"path/filepath"
	"strings"
	"testing"

	"gozart/pkg/config"
	"gozart/pkg/logger"
	"gozart/pkg/storage"
)

func setupTestServer(t *testing.T) (*Server, *storage.DB) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test_web.db")

	db, err := storage.New(dbPath)
	if err != nil {
		t.Fatalf("failed to init db: %v", err)
	}

	l := logger.New()
	cfg := &config.ConfigLoader{
		Pwd:    tempDir,
		Logger: l,
	}
	cfg.ConfigData.Storage.DBPath = dbPath
	cfg.ConfigData.Storage.WorkDir = filepath.Join(tempDir, "workspace")
	cfg.ConfigData.Storage.OutputDir = filepath.Join(tempDir, "output")
	cfg.ConfigData.Server.Port = 8080
	cfg.ConfigData.Server.Host = "127.0.0.1"

	server := NewServer(cfg, db)
	return server, db
}

func TestWebHandlers(t *testing.T) {
	server, db := setupTestServer(t)
	defer db.Close()

	// 1. Dashboard GET /
	req := httptest.NewRequest("GET", "/", nil)
	rr := httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET / returned status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Videos") {
		t.Errorf("GET / missing 'Videos'")
	}

	// 2. New Video GET /new
	req = httptest.NewRequest("GET", "/new", nil)
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /new returned status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "New Video") {
		t.Errorf("GET /new missing 'New Video'")
	}

	// 3. Settings GET /settings
	req = httptest.NewRequest("GET", "/settings", nil)
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /settings returned status %d, want %d", rr.Code, http.StatusOK)
	}
	if !strings.Contains(rr.Body.String(), "Accounts, models and tools") {
		t.Errorf("GET /settings missing settings subtitle")
	}

	// 4. Create Job POST /jobs/create
	form := url.Values{}
	form.Set("topic", "Why Rome Fell")
	form.Set("country_code", "GB")
	form.Set("format", "shorts_9_16")
	form.Set("tone", "educational")
	form.Set("target_duration_sec", "45")

	req = httptest.NewRequest("POST", "/jobs/create", strings.NewReader(form.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)

	if rr.Code != http.StatusSeeOther {
		t.Fatalf("POST /jobs/create returned %d, want 303 SeeOther", rr.Code)
	}

	loc := rr.Header().Get("Location")
	if !strings.HasPrefix(loc, "/jobs/") {
		t.Fatalf("unexpected redirect location: %s", loc)
	}

	// 5. Inspect Job GET /jobs/{id}
	req = httptest.NewRequest("GET", loc, nil)
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET %s returned status %d, want 200", loc, rr.Code)
	}
	if !strings.Contains(rr.Body.String(), "Why Rome Fell") {
		t.Errorf("GET %s missing topic 'Why Rome Fell'", loc)
	}

	// 6. Static files
	req = httptest.NewRequest("GET", "/static/css/tailwind.css", nil)
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /static/css/tailwind.css returned %d", rr.Code)
	}

	req = httptest.NewRequest("GET", "/static/js/htmx.min.js", nil)
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Errorf("GET /static/js/htmx.min.js returned %d", rr.Code)
	}

	// 7. Settings Save POST /settings/save
	settingsForm := url.Values{}
	settingsForm.Set("agent_model", "test-agent:v1")
	settingsForm.Set("fast_model", "test-fast:v1")
	settingsForm.Set("ai_provider", "ollama")
	settingsForm.Set("tts_engine", "edge-tts")

	req = httptest.NewRequest("POST", "/settings/save", strings.NewReader(settingsForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /settings/save returned %d, want 303", rr.Code)
	}
	if server.Config.ConfigData.AI.AgentModel != "test-agent:v1" {
		t.Errorf("expected updated agent model test-agent:v1, got %s", server.Config.ConfigData.AI.AgentModel)
	}

	// 8. YouTube Auth GET /api/auth/youtube
	req = httptest.NewRequest("GET", "/api/auth/youtube", nil)
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("GET /api/auth/youtube returned %d, want 303 redirect", rr.Code)
	}

	// 9. Job Publish POST /api/jobs/{id}/publish with custom metadata
	jobID := strings.TrimPrefix(loc, "/jobs/")
	publishForm := url.Values{}
	publishForm.Set("title", "Custom Title For Rome")
	publishForm.Set("description", "Custom breakdown of the collapse")
	publishForm.Set("tags", "rome, history, documentary")

	req = httptest.NewRequest("POST", "/api/jobs/"+jobID+"/publish", strings.NewReader(publishForm.Encode()))
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	rr = httptest.NewRecorder()
	server.Router.ServeHTTP(rr, req)
	if rr.Code != http.StatusSeeOther {
		t.Errorf("POST /api/jobs/%s/publish returned %d, want 303", jobID, rr.Code)
	}

	// 10. SSE Broker with progress
	ch := server.SSEBroker.Subscribe(jobID)
	server.SSEBroker.BroadcastWithProgress(jobID, "INFO", "Testing SSE with progress", 75, "TESTING")
	select {
	case event := <-ch:
		if event.ProgressPct != 75 || event.CurrentStep != "TESTING" {
			t.Errorf("unexpected SSE event: %+v", event)
		}
	default:
		t.Errorf("expected SSE event on subscriber channel")
	}
	server.SSEBroker.Unsubscribe(jobID, ch)
}
