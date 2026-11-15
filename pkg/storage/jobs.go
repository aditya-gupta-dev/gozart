package storage

import (
	"database/sql"
	"fmt"
	"time"
)

type Job struct {
	ID             string    `json:"id"`
	Topic          string    `json:"topic"`
	CountryCode    string    `json:"country_code"`
	Format         string    `json:"format"`
	Status         string    `json:"status"`
	CurrentStep    string    `json:"current_step"`
	ProgressPct    int       `json:"progress_pct"`
	ScriptJSON     string    `json:"script_json"`
	SentimentJSON  string    `json:"sentiment_json"`
	SEOJSON        string    `json:"seo_json"`
	VideoPath      string    `json:"video_path"`
	ThumbnailPath  string    `json:"thumbnail_path"`
	YouTubeVideoID string    `json:"youtube_video_id"`
	ErrorMessage   string    `json:"error_message"`
	CreatedAt      time.Time `json:"created_at"`
	UpdatedAt      time.Time `json:"updated_at"`
}

type JobLog struct {
	ID        int64     `json:"id"`
	JobID     string    `json:"job_id"`
	Level     string    `json:"level"`
	Message   string    `json:"message"`
	CreatedAt time.Time `json:"created_at"`
}

type JobStats struct {
	Total    int `json:"total"`
	Uploaded int `json:"uploaded"`
	Active   int `json:"active"`
}

func (d *DB) CreateJob(j *Job) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	now := time.Now().UTC()
	j.CreatedAt = now
	j.UpdatedAt = now
	if j.Status == "" {
		j.Status = StatusQueued
	}
	if j.CurrentStep == "" {
		j.CurrentStep = StateInit
	}

	query := `
	INSERT INTO jobs (
		id, topic, country_code, format, status, current_step, progress_pct,
		script_json, sentiment_json, seo_json, video_path, thumbnail_path,
		youtube_video_id, error_message, created_at, updated_at
	) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`

	_, err := d.db.Exec(query,
		j.ID, j.Topic, j.CountryCode, j.Format, j.Status, j.CurrentStep, j.ProgressPct,
		j.ScriptJSON, j.SentimentJSON, j.SEOJSON, j.VideoPath, j.ThumbnailPath,
		j.YouTubeVideoID, j.ErrorMessage, j.CreatedAt, j.UpdatedAt,
	)
	if err != nil {
		return fmt.Errorf("failed to create job: %w", err)
	}
	return nil
}

func (d *DB) GetJob(id string) (*Job, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `
	SELECT id, topic, country_code, format, status, current_step, progress_pct,
	       COALESCE(script_json, ''), COALESCE(sentiment_json, ''), COALESCE(seo_json, ''),
	       COALESCE(video_path, ''), COALESCE(thumbnail_path, ''), COALESCE(youtube_video_id, ''),
	       COALESCE(error_message, ''), created_at, updated_at
	FROM jobs WHERE id = ?`

	row := d.db.QueryRow(query, id)
	j := &Job{}
	err := row.Scan(
		&j.ID, &j.Topic, &j.CountryCode, &j.Format, &j.Status, &j.CurrentStep, &j.ProgressPct,
		&j.ScriptJSON, &j.SentimentJSON, &j.SEOJSON,
		&j.VideoPath, &j.ThumbnailPath, &j.YouTubeVideoID,
		&j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt,
	)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("failed to get job: %w", err)
	}
	return j, nil
}

func (d *DB) ListJobs(limit int) ([]*Job, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	if limit <= 0 {
		limit = 50
	}

	query := `
	SELECT id, topic, country_code, format, status, current_step, progress_pct,
	       COALESCE(script_json, ''), COALESCE(sentiment_json, ''), COALESCE(seo_json, ''),
	       COALESCE(video_path, ''), COALESCE(thumbnail_path, ''), COALESCE(youtube_video_id, ''),
	       COALESCE(error_message, ''), created_at, updated_at
	FROM jobs ORDER BY created_at DESC LIMIT ?`

	rows, err := d.db.Query(query, limit)
	if err != nil {
		return nil, fmt.Errorf("failed to list jobs: %w", err)
	}
	defer rows.Close()

	var jobs []*Job
	for rows.Next() {
		j := &Job{}
		err := rows.Scan(
			&j.ID, &j.Topic, &j.CountryCode, &j.Format, &j.Status, &j.CurrentStep, &j.ProgressPct,
			&j.ScriptJSON, &j.SentimentJSON, &j.SEOJSON,
			&j.VideoPath, &j.ThumbnailPath, &j.YouTubeVideoID,
			&j.ErrorMessage, &j.CreatedAt, &j.UpdatedAt,
		)
		if err != nil {
			return nil, fmt.Errorf("failed to scan job: %w", err)
		}
		jobs = append(jobs, j)
	}
	return jobs, nil
}

func (d *DB) UpdateJobProgress(id string, step string, progressPct int) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE jobs SET current_step = ?, progress_pct = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, step, progressPct, time.Now().UTC(), id)
	return err
}

func (d *DB) UpdateJobStatus(id string, status string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE jobs SET status = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, status, time.Now().UTC(), id)
	return err
}

func (d *DB) UpdateJobScript(id string, scriptJSON string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE jobs SET script_json = ?, current_step = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, scriptJSON, StateScriptDone, time.Now().UTC(), id)
	return err
}

func (d *DB) UpdateJobSentiment(id string, sentimentJSON string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE jobs SET sentiment_json = ?, current_step = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, sentimentJSON, StateSentimentDone, time.Now().UTC(), id)
	return err
}

func (d *DB) UpdateJobSEO(id string, seoJSON string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE jobs SET seo_json = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, seoJSON, time.Now().UTC(), id)
	return err
}

func (d *DB) UpdateJobOutputs(id string, videoPath, thumbPath string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE jobs SET video_path = ?, thumbnail_path = ?, current_step = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, videoPath, thumbPath, StateComposited, time.Now().UTC(), id)
	return err
}

func (d *DB) UpdateJobYouTubeID(id string, ytID string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE jobs SET youtube_video_id = ?, current_step = ?, status = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, ytID, StateUploaded, StatusCompleted, time.Now().UTC(), id)
	return err
}

func (d *DB) SetJobError(id string, errMsg string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `UPDATE jobs SET status = ?, error_message = ?, updated_at = ? WHERE id = ?`
	_, err := d.db.Exec(query, StatusFailed, errMsg, time.Now().UTC(), id)
	return err
}

func (d *DB) AddJobLog(jobID, level, message string) error {
	d.mu.Lock()
	defer d.mu.Unlock()

	query := `INSERT INTO job_logs (job_id, level, message, created_at) VALUES (?, ?, ?, ?)`
	_, err := d.db.Exec(query, jobID, level, message, time.Now().UTC())
	return err
}

func (d *DB) GetJobLogs(jobID string) ([]*JobLog, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	query := `SELECT id, job_id, level, message, created_at FROM job_logs WHERE job_id = ? ORDER BY id ASC`
	rows, err := d.db.Query(query, jobID)
	if err != nil {
		return nil, fmt.Errorf("failed to query logs: %w", err)
	}
	defer rows.Close()

	var logs []*JobLog
	for rows.Next() {
		l := &JobLog{}
		if err := rows.Scan(&l.ID, &l.JobID, &l.Level, &l.Message, &l.CreatedAt); err != nil {
			return nil, err
		}
		logs = append(logs, l)
	}
	return logs, nil
}

func (d *DB) GetJobStats() (JobStats, error) {
	d.mu.RLock()
	defer d.mu.RUnlock()

	var stats JobStats
	err := d.db.QueryRow(`SELECT COUNT(*) FROM jobs`).Scan(&stats.Total)
	if err != nil {
		return stats, err
	}
	err = d.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE youtube_video_id != ''`).Scan(&stats.Uploaded)
	if err != nil {
		return stats, err
	}
	err = d.db.QueryRow(`SELECT COUNT(*) FROM jobs WHERE status IN ('QUEUED', 'RUNNING')`).Scan(&stats.Active)
	if err != nil {
		return stats, err
	}
	return stats, nil
}
