CREATE TABLE IF NOT EXISTS jobs (
    id TEXT PRIMARY KEY,
    topic TEXT NOT NULL,
    country_code TEXT NOT NULL,
    format TEXT NOT NULL,
    status TEXT NOT NULL,
    current_step TEXT NOT NULL,
    progress_pct INTEGER DEFAULT 0,
    script_json TEXT,
    sentiment_json TEXT,
    seo_json TEXT,
    video_path TEXT,
    thumbnail_path TEXT,
    youtube_video_id TEXT,
    error_message TEXT,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    updated_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP
);

CREATE TABLE IF NOT EXISTS job_logs (
    id INTEGER PRIMARY KEY AUTOINCREMENT,
    job_id TEXT NOT NULL,
    level TEXT NOT NULL,
    message TEXT NOT NULL,
    created_at TIMESTAMP DEFAULT CURRENT_TIMESTAMP,
    FOREIGN KEY(job_id) REFERENCES jobs(id)
);
