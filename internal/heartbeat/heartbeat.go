package heartbeat

import (
	"context"
	"fmt"
	"log"
	"net/http"
	"time"
)

// Service handles the periodic ping to the Orchestrator.
type Service struct {
	orchestratorURL string
	interval        time.Duration
	workerID        string
	client          *http.Client
}

// New creates a heartbeat service. We initialize a custom HTTP client 
// with a timeout so a slow Udoo board doesn't hang our worker.
func New(url string, intervalSec int, workerID string) *Service {
	return &Service{
		orchestratorURL: url,
		interval:        time.Duration(intervalSec) * time.Second,
		workerID:        workerID,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

// Start launches the heartbeat loop in a non-blocking way.
func (s *Service) Start(ctx context.Context) {
	// Create a ticker that fires every 'interval'
	ticker := time.NewTicker(s.interval)
	
	// 'go' keyword starts this function in a background goroutine
	go func() {
		defer ticker.Stop() // Cleanup when the routine exits
		log.Printf("Heartbeat started for %s", s.workerID)

		for {
			select {
			case <-ctx.Done():
				// If the app shuts down, this channel closes
				log.Println("Stopping heartbeat...")
				return
			case <-ticker.C:
				// Every tick, execute the ping
				s.ping()
			}
		}
	}()
}

func (s *Service) ping() {
	// For now, i'm using a simple GET. Later, i'll change this to POST 
	// to send hardware stats (CPU usage, etc.)
	fullURL := fmt.Sprintf("%s/api/heartbeat?id=%s", s.orchestratorURL, s.workerID)
	
	resp, err := s.client.Get(fullURL)
	if err != nil {
		log.Printf("[Heartbeat] Failed: %v", err)
		return
	}
	defer resp.Body.Close() 

	if resp.StatusCode != http.StatusOK {
		log.Printf("[Heartbeat] Orchestrator returned status: %d", resp.StatusCode)
	}
}