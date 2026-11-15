package storage

import (
	"path/filepath"
	"testing"
)

func TestStorageLifecycle(t *testing.T) {
	tempDir := t.TempDir()
	dbPath := filepath.Join(tempDir, "test.db")

	db, err := New(dbPath)
	if err != nil {
		t.Fatalf("failed to create db: %v", err)
	}
	defer db.Close()

	job := &Job{
		ID:          "job-123",
		Topic:       "NASA Moon Mission",
		CountryCode: "US",
		Format:      "shorts_9_16",
	}

	if err := db.CreateJob(job); err != nil {
		t.Fatalf("failed to create job: %v", err)
	}

	retrieved, err := db.GetJob("job-123")
	if err != nil {
		t.Fatalf("failed to get job: %v", err)
	}
	if retrieved == nil {
		t.Fatalf("expected job-123 to exist")
	}
	if retrieved.Topic != "NASA Moon Mission" {
		t.Errorf("expected topic NASA Moon Mission, got %s", retrieved.Topic)
	}
	if retrieved.CurrentStep != StateInit {
		t.Errorf("expected state %s, got %s", StateInit, retrieved.CurrentStep)
	}

	// Test progress update
	if err := db.UpdateJobProgress("job-123", StateResearchDone, 25); err != nil {
		t.Fatalf("failed to update progress: %v", err)
	}

	// Test logging
	if err := db.AddJobLog("job-123", "INFO", "Research completed successfully"); err != nil {
		t.Fatalf("failed to add log: %v", err)
	}

	logs, err := db.GetJobLogs("job-123")
	if err != nil {
		t.Fatalf("failed to get logs: %v", err)
	}
	if len(logs) != 1 {
		t.Fatalf("expected 1 log, got %d", len(logs))
	}
	if logs[0].Message != "Research completed successfully" {
		t.Errorf("expected message 'Research completed successfully', got %s", logs[0].Message)
	}

	// Test stats
	stats, err := db.GetJobStats()
	if err != nil {
		t.Fatalf("failed to get stats: %v", err)
	}
	if stats.Total != 1 {
		t.Errorf("expected 1 total job, got %d", stats.Total)
	}
}
