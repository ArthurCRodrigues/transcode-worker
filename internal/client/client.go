package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"net/http"
	"time"

	"github.com/hashicorp/go-retryablehttp"
	"transcode-worker/internal/config"
	"transcode-worker/pkg/models"
)

type OrchestratorClient struct {
	baseURL    string
	workerID   string
	httpClient *http.Client
}

// NewOrchestratorClient creates a robust HTTP client with retries
func NewOrchestratorClient(cfg *config.Config) *OrchestratorClient {
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	retryClient.Logger = nil // Silence default debug logger

	return &OrchestratorClient{
		baseURL:    cfg.OrchestratorURL,
		workerID:   cfg.WorkerID,
		httpClient: retryClient.StandardClient(),
	}
}

// doRequest is the core HTTP request handler with error interception
func (c *OrchestratorClient) doRequest(ctx context.Context, method, path string, payload interface{}, response interface{}) error {
	url := fmt.Sprintf("%s%s", c.baseURL, path)

	var body io.Reader
	if payload != nil {
		jsonBytes, err := json.Marshal(payload)
		if err != nil {
			return fmt.Errorf("failed to marshal payload: %w", err)
		}
		body = bytes.NewBuffer(jsonBytes)
	}

	req, err := http.NewRequestWithContext(ctx, method, url, body)
	if err != nil {
		return fmt.Errorf("failed to create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Worker-ID", c.workerID)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("request failed: %w", err)
	}
	defer resp.Body.Close()

	// Handle 404 - Orchestrator lost worker state (needs re-registration)
	if resp.StatusCode == http.StatusNotFound {
		return &OrchestratorStateError{StatusCode: resp.StatusCode}
	}

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned error status: %d", resp.StatusCode)
	}

	// Decode response if expected
	if response != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// OrchestratorStateError indicates the orchestrator lost worker state
type OrchestratorStateError struct {
	StatusCode int
}

func (e *OrchestratorStateError) Error() string {
	return fmt.Sprintf("orchestrator state error: status %d", e.StatusCode)
}

// ===== Worker Registration =====

// Register declares the worker's capabilities to the orchestrator
// Called once on startup and automatically on state loss recovery
func (c *OrchestratorClient) Register(ctx context.Context, capabilities models.WorkerCapabilities) error {
	payload := models.RegistrationPayload{
		WorkerID:     c.workerID,
		Capabilities: capabilities,
	}

	log.Printf("Registering worker with orchestrator...")
	if err := c.doRequest(ctx, "POST", "/api/v1/workers/register", payload, nil); err != nil {
		return fmt.Errorf("registration failed: %w", err)
	}

	log.Printf("Successfully registered worker: %s", c.workerID)
	return nil
}

// ===== Worker Sync (Bidirectional Heartbeat + Job Assignment) =====

// Sync sends worker state and receives potential job assignment
// This replaces the old separate Heartbeat + RequestJob pattern
func (c *OrchestratorClient) Sync(ctx context.Context, payload models.SyncPayload) (*models.SyncResponse, error) {
	var syncResp models.SyncResponse

	err := c.doRequest(ctx, "POST", "/api/v1/workers/sync", payload, &syncResp)
	if err != nil {
		// Check if it's a state error (orchestrator restart)
		if stateErr, ok := err.(*OrchestratorStateError); ok {
			return nil, stateErr
		}
		return nil, fmt.Errorf("sync failed: %w", err)
	}

	return &syncResp, nil
}

// ===== Job Status Updates =====

// UpdateJobStatus reports transcoding progress
func (c *OrchestratorClient) UpdateJobStatus(ctx context.Context, jobID string, payload models.JobStatusPayload) error {
	path := fmt.Sprintf("/api/v1/jobs/%s", jobID)
	return c.doRequest(ctx, "PATCH", path, payload, nil)
}

// FinalizeJob reports job completion or failure
func (c *OrchestratorClient) FinalizeJob(ctx context.Context, jobID string, payload models.JobResultPayload) error {
	path := fmt.Sprintf("/api/v1/jobs/%s/finalize", jobID)
	return c.doRequest(ctx, "POST", path, payload, nil)
}
