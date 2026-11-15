package handlers

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sync"
	"time"

	"github.com/go-chi/chi/v5"
)

type SSEEvent struct {
	Time        string `json:"time"`
	Level       string `json:"level"`
	Message     string `json:"message"`
	ProgressPct int    `json:"progress_pct,omitempty"`
	CurrentStep string `json:"current_step,omitempty"`
}

type SSEBroker struct {
	mu          sync.RWMutex
	subscribers map[string][]chan SSEEvent
}

func NewSSEBroker() *SSEBroker {
	return &SSEBroker{
		subscribers: make(map[string][]chan SSEEvent),
	}
}

func (b *SSEBroker) Subscribe(jobID string) chan SSEEvent {
	b.mu.Lock()
	defer b.mu.Unlock()

	ch := make(chan SSEEvent, 50)
	b.subscribers[jobID] = append(b.subscribers[jobID], ch)
	return ch
}

func (b *SSEBroker) Unsubscribe(jobID string, ch chan SSEEvent) {
	b.mu.Lock()
	defer b.mu.Unlock()

	subs := b.subscribers[jobID]
	for i, sub := range subs {
		if sub == ch {
			b.subscribers[jobID] = append(subs[:i], subs[i+1:]...)
			close(ch)
			break
		}
	}
}

func (b *SSEBroker) Broadcast(jobID, level, message string) {
	b.BroadcastWithProgress(jobID, level, message, 0, "")
}

func (b *SSEBroker) BroadcastWithProgress(jobID, level, message string, progressPct int, currentStep string) {
	b.mu.RLock()
	defer b.mu.RUnlock()

	event := SSEEvent{
		Time:        time.Now().Format("15:04:05"),
		Level:       level,
		Message:     message,
		ProgressPct: progressPct,
		CurrentStep: currentStep,
	}

	for _, ch := range b.subscribers[jobID] {
		select {
		case ch <- event:
		default:
		}
	}
}

func (b *SSEBroker) HandleStream(w http.ResponseWriter, r *http.Request) {
	jobID := chi.URLParam(r, "id")
	if jobID == "" {
		http.Error(w, "missing job id", http.StatusBadRequest)
		return
	}

	flusher, ok := w.(http.Flusher)
	if !ok {
		http.Error(w, "streaming unsupported", http.StatusInternalServerError)
		return
	}

	w.Header().Set("Content-Type", "text/event-stream")
	w.Header().Set("Cache-Control", "no-cache")
	w.Header().Set("Connection", "keep-alive")
	w.Header().Set("Access-Control-Allow-Origin", "*")

	ch := b.Subscribe(jobID)
	defer b.Unsubscribe(jobID, ch)

	// Send initial connect frame
	fmt.Fprintf(w, "data: %s\n\n", `{"level":"INFO","message":"Connected to live event stream"}`)
	flusher.Flush()

	notify := r.Context().Done()
	for {
		select {
		case <-notify:
			return
		case event, ok := <-ch:
			if !ok {
				return
			}
			data, _ := json.Marshal(event)
			fmt.Fprintf(w, "data: %s\n\n", string(data))
			flusher.Flush()
		}
	}
}
