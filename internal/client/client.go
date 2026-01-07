package client

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
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

// NewOrchestratorClient creates a robust HTTP client with retries.
func NewOrchestratorClient(cfg *config.Config) *OrchestratorClient {
	// Create a retryable client
	retryClient := retryablehttp.NewClient()
	retryClient.RetryMax = 3
	retryClient.RetryWaitMin = 1 * time.Second
	retryClient.RetryWaitMax = 5 * time.Second
	
	// Silence the default debug logger for cleaner output
	retryClient.Logger = nil 

	return &OrchestratorClient{
		baseURL:    cfg.OrchestratorURL,
		workerID:   cfg.WorkerID,
		httpClient: retryClient.StandardClient(),
	}
}

// Helper generic method to handle requests
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

	if resp.StatusCode >= 400 {
		return fmt.Errorf("API returned error status: %d", resp.StatusCode)
	}

	// If we expect a response body and provided a struct to unmarshal into
	if response != nil && resp.StatusCode != http.StatusNoContent {
		if err := json.NewDecoder(resp.Body).Decode(response); err != nil {
			return fmt.Errorf("failed to decode response: %w", err)
		}
	}

	return nil
}

// --- Implementation of Interface Methods ---

func (c *OrchestratorClient) Register(ctx context.Context, workerID string) error {
	// Simple ping to register presence
	return c.doRequest(ctx, "POST", "/api/v1/workers/register", map[string]string{"worker_id": workerID}, nil)
}

func (c *OrchestratorClient) Heartbeat(ctx context.Context, payload models.HeartbeatPayload) error {
	return c.doRequest(ctx, "POST", "/api/v1/workers/heartbeat", payload, nil)
}

func (c *OrchestratorClient) RequestJob(ctx context.Context, payload models.JobRequestPayload) (*models.JobSpec, error) {
	var jobResp models.JobSpec
	
	err := c.doRequest(ctx, "POST", "/api/v1/jobs/request", payload, &jobResp)
	if err != nil {
		// Differentiate between "Network Error" and "No Jobs Available"
		// Assuming 404 means no jobs, or the API returns 204 No Content
		return nil, err
	}

	// Check if the received JobID is empty (logic depends on your API design)
	if jobResp.JobID == "" {
		return nil, nil
	}

	return &jobResp, nil
}

func (c *OrchestratorClient) UpdateJobStatus(ctx context.Context, jobID string, payload models.JobStatusPayload) error {
	path := fmt.Sprintf("/api/v1/jobs/%s", jobID)
	return c.doRequest(ctx, "PATCH", path, payload, nil)
}

func (c *OrchestratorClient) FinalizeJob(ctx context.Context, jobID string, payload models.JobResultPayload) error {
	path := fmt.Sprintf("/api/v1/jobs/%s/finalize", jobID)
	return c.doRequest(ctx, "POST", path, payload, nil)
}