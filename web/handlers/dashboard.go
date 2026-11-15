package handlers

import (
	"html/template"
	"net/http"
	"os"

	"gozart/pkg/config"
	"gozart/pkg/storage"
	"gozart/web"
)

type DashboardHandler struct {
	DB     *storage.DB
	Config *config.ConfigLoader
}

func NewDashboardHandler(db *storage.DB, cfg *config.ConfigLoader) *DashboardHandler {
	return &DashboardHandler{DB: db, Config: cfg}
}

func (h *DashboardHandler) HandleDashboard(w http.ResponseWriter, r *http.Request) {
	stats, err := h.DB.GetJobStats()
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	jobs, err := h.DB.ListJobs(30)
	if err != nil {
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	tmpl, err := template.ParseFS(web.EmbeddedFiles, "templates/base.html", "templates/dashboard.html")
	if err != nil {
		http.Error(w, "template parsing error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":     "Mission Control",
		"ActiveNav": "dashboard",
		"Stats":     stats,
		"Jobs":      jobs,
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "dashboard.html", data)
}

func (h *DashboardHandler) HandleSettings(w http.ResponseWriter, r *http.Request) {
	tokenPath := h.Config.ConfigData.YouTube.TokenCachePath
	if tokenPath == "" {
		tokenPath = "token.json"
	}
	_, tokenErr := os.Stat(tokenPath)
	hasToken := tokenErr == nil

	secretsPath := h.Config.ConfigData.YouTube.ClientSecretsPath
	if secretsPath == "" {
		secretsPath = "client_secrets.json"
	}
	_, secretsErr := os.Stat(secretsPath)
	hasSecrets := secretsErr == nil

	tmpl, err := template.ParseFS(web.EmbeddedFiles, "templates/base.html", "templates/settings.html")
	if err != nil {
		http.Error(w, "template parsing error: "+err.Error(), http.StatusInternalServerError)
		return
	}

	data := map[string]interface{}{
		"Title":      "Settings",
		"ActiveNav":  "settings",
		"Config":     h.Config.ConfigData,
		"HasToken":   hasToken,
		"HasSecrets": hasSecrets,
		"Saved":      r.URL.Query().Get("saved") == "true",
		"AuthMsg":    r.URL.Query().Get("auth_msg"),
	}

	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	_ = tmpl.ExecuteTemplate(w, "settings.html", data)
}

func (h *DashboardHandler) HandleSettingsSave(w http.ResponseWriter, r *http.Request) {
	if err := r.ParseForm(); err != nil {
		http.Error(w, "invalid form data", http.StatusBadRequest)
		return
	}

	cfg := &h.Config.ConfigData

	if p := r.FormValue("ai_provider"); p != "" {
		cfg.AI.Provider = p
	}
	if u := r.FormValue("ollama_base_url"); u != "" {
		cfg.AI.OllamaBaseURL = u
	}
	if m := r.FormValue("agent_model"); m != "" {
		cfg.AI.AgentModel = m
	}
	if fm := r.FormValue("fast_model"); fm != "" {
		cfg.AI.FastModel = fm
	}
	if k := r.FormValue("ai_api_key"); k != "" {
		cfg.AI.APIKey = k
	}
	if yk := r.FormValue("youtube_api_key"); yk != "" {
		cfg.YouTube.APIKey = yk
		cfg.YoutubeAPIKey = yk
	}
	if te := r.FormValue("tts_engine"); te != "" {
		cfg.Audio.TTSEngine = te
	}
	if tv := r.FormValue("voice"); tv != "" {
		cfg.Audio.Voice = tv
	}
	if df := r.FormValue("deepfilter_path"); df != "" {
		cfg.Audio.DeepFilterPath = df
	}
	if me := r.FormValue("media_engine"); me != "" {
		cfg.Media.Engine = me
	}
	if nb := r.FormValue("node_binary"); nb != "" {
		cfg.Media.NodeBinary = nb
	}
	if db := r.FormValue("db_path"); db != "" {
		cfg.Storage.DBPath = db
	}

	h.Config.ConfigData = *cfg
	_ = h.Config.SaveConfig()

	http.Redirect(w, r, "/settings?saved=true", http.StatusSeeOther)
}

func (h *DashboardHandler) HandleYouTubeAuth(w http.ResponseWriter, r *http.Request) {
	secretsPath := h.Config.ConfigData.YouTube.ClientSecretsPath
	if secretsPath == "" {
		secretsPath = "client_secrets.json"
	}

	if _, err := os.Stat(secretsPath); os.IsNotExist(err) {
		http.Redirect(w, r, "/settings?auth_msg=client_secrets_missing", http.StatusSeeOther)
		return
	}

	tokenPath := h.Config.ConfigData.YouTube.TokenCachePath
	if tokenPath == "" {
		tokenPath = "token.json"
	}

	if _, err := os.Stat(tokenPath); err == nil {
		http.Redirect(w, r, "/settings?auth_msg=token_already_active", http.StatusSeeOther)
		return
	}

	http.Redirect(w, r, "/settings?auth_msg=reauth_required", http.StatusSeeOther)
}
