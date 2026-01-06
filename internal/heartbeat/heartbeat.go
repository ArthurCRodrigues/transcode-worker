package heartbeat

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"time"
	"transcode-worker/internal/transcoder"
	"transcode-worker/pkg/models"
)

type Service struct {
	orchestratorURL string
	interval        time.Duration
	workerID        string
	client          *http.Client
	engine          *transcoder.Engine
}

func New(url string, intervalSec int, workerID string, engine *transcoder.Engine) *Service {
	return &Service{
		orchestratorURL: url,
		interval:        time.Duration(intervalSec) * time.Second,
		workerID:        workerID,
		engine:          engine,
		client: &http.Client{
			Timeout: 5 * time.Second,
		},
	}
}

func (s *Service) Start(ctx context.Context) {
	ticker := time.NewTicker(s.interval)
	
	// Perform initial registration
	s.register()

	go func() {
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.sendPulse()
			}
		}
	}()
}

func (s *Service) register() {
	specs := s.engine.GetStaticSpecs()
	reg := models.WorkerRegistration{
		ID:          s.workerID,
		BaseURL:     "", // Should be populated from config
		StaticSpecs: specs,
	}

	s.post("/v1/workers", reg)
}

func (s *Service) sendPulse() {
	health := s.engine.GetSystemHealth()
	
	hb := models.Heartbeat{
		Status:    "IDLE", // Later: get from Scheduler
		Telemetry: health,
	}

	path := fmt.Sprintf("/v1/workers/%s/heartbeats", s.workerID)
	s.post(path, hb)
}

func (s *Service) post(path string, data interface{}) {
	url := s.orchestratorURL + path
	body, _ := json.Marshal(data)
	
	resp, err := s.client.Post(url, "application/json", bytes.NewBuffer(body))
	if err != nil {
		log.Printf("[Heartbeat] Request failed: %v", err)
		return
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		log.Printf("[Heartbeat] Orchestrator error: %d", resp.StatusCode)
	}
}