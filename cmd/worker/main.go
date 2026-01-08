package main

import (
	"context"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"path/filepath"
	"strings"
	"sync"
	"syscall"
	"time"

	"transcode-worker/internal/client"
	"transcode-worker/internal/config"
	"transcode-worker/internal/monitor"
	"transcode-worker/internal/transcoder"
	"transcode-worker/pkg/models"
)

type Worker struct {
	cfg          *config.Config
	client       *client.OrchestratorClient
	monitor      *monitor.SystemMonitor
	transcoder   *transcoder.FFmpegTranscoder
	capabilities []string
	
	currentJob *models.JobSpec
	jobMutex   sync.Mutex
	cancelJob  context.CancelFunc
	
	shutdownCh chan struct{}
	wg         sync.WaitGroup
}

func main() {
	// Load configuration
	cfg, err := config.Load(".")
	if err != nil {
		slog.Error("Failed to load config", "error", err)
		os.Exit(1)
	}

	// Configure structured logging
	var level slog.Level
	switch cfg.LogLevel {
	case "debug":
		level = slog.LevelDebug
	case "warn":
		level = slog.LevelWarn
	case "error":
		level = slog.LevelError
	default:
		level = slog.LevelInfo
	}

	// Use JSON handler for production (machine-readable)
	// For development, you can use NewTextHandler instead
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
	slog.SetDefault(logger)

	slog.Info("Starting transcode worker",
		"worker_id", cfg.WorkerID,
		"orchestrator_url", cfg.OrchestratorURL,
		"nas_mount_path", cfg.NasMountPath,
		"temp_dir", cfg.TempDir,
		"log_level", cfg.LogLevel)

	// Initialize components
	orchestratorClient := client.NewOrchestratorClient(cfg)
	systemMonitor := monitor.NewSystemMonitor()
	ffmpegTranscoder := transcoder.NewTranscoder(cfg.TempDir)

	worker := &Worker{
		cfg:        cfg,
		client:     orchestratorClient,
		monitor:    systemMonitor,
		transcoder: ffmpegTranscoder,
		shutdownCh: make(chan struct{}),
	}

	// Discover capabilities once at startup
	ctx := context.Background()
	caps, err := systemMonitor.GetCapabilities(ctx)
	if err != nil {
		slog.Error("Failed to discover capabilities", "error", err)
		os.Exit(1)
	}

	worker.capabilities = caps

	slog.Info("Discovered capabilities", "capabilities", caps)

	// Initial registration
	if err := worker.register(); err != nil {
		slog.Error("Failed to register with orchestrator", "error", err)
		os.Exit(1)
	}

	// Handle graceful shutdown
	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	// Start sync loop (replaces heartbeat + job polling)
	worker.wg.Add(1)
	go worker.syncLoop()

	// Wait for shutdown signal
	<-sigCh
	slog.Info("Shutdown signal received, cleaning up...")
	
	worker.shutdown()
	
	slog.Info("Worker stopped gracefully")
}

// register declares the worker's capabilities to the orchestrator
func (w *Worker) register() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	return w.client.Register(ctx, w.capabilities)
}

// syncLoop is the unified heartbeat + job polling loop
func (w *Worker) syncLoop() {
	defer w.wg.Done()
	
	ticker := time.NewTicker(w.cfg.SyncInterval)
	defer ticker.Stop()
	
	slog.Info("Starting sync loop", "interval", w.cfg.SyncInterval)
	
	for {
		select {
		case <-ticker.C:
			if err := w.performSync(); err != nil {
				slog.Error("Sync failed", "error", err)
			}
			
		case <-w.shutdownCh:
			slog.Info("Sync loop stopping...")
			return
		}
	}
}

// performSync sends worker state and receives potential job assignment
func (w *Worker) performSync() error {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	// Get current hardware stats
	stats, err := w.monitor.GetStats(ctx)
	if err != nil {
		return fmt.Errorf("failed to get stats: %w", err)
	}
	
	// Determine current status
	w.jobMutex.Lock()
	status := "IDLE"
	currentJobID := ""
	if w.currentJob != nil {
		status = "BUSY"
		currentJobID = w.currentJob.JobID
	} else if stats.IsBusy {
		status = "BUSY" // System under load from other processes
	}
	w.jobMutex.Unlock()
	
	// Build sync payload
	payload := models.SyncPayload{
		WorkerID:      w.cfg.WorkerID,
		Status:        status,
		HardwareStats: stats,
		CurrentJobID:  currentJobID,
	}
	
	// Send sync request
	syncResp, err := w.client.Sync(ctx, payload)
	if err != nil {
		// Check if orchestrator lost state (needs re-registration)
		if _, isStateError := err.(*client.OrchestratorStateError); isStateError {
			slog.Warn("Orchestrator lost state, re-registering...")
			if regErr := w.register(); regErr != nil {
				return fmt.Errorf("re-registration failed: %w", regErr)
			}
			// Retry the sync after re-registration
			syncResp, err = w.client.Sync(ctx, payload)
			if err != nil {
				return fmt.Errorf("sync retry after registration failed: %w", err)
			}
		} else {
			return err
		}
	}
	
	// Handle job assignment if worker is IDLE
	if syncResp.AssignedJob != nil {
		w.jobMutex.Lock()
		isIdle := w.currentJob == nil
		w.jobMutex.Unlock()
		
		if !isIdle {
			slog.Warn("Rejecting job assignment - already processing",
				"assigned_job", syncResp.AssignedJob.JobID,
				"current_job", w.currentJob.JobID)
			return nil
		}
		
		slog.Info("Received job assignment", "job_id", syncResp.AssignedJob.JobID)
		
		// Resolve paths and execute job
		if err := w.resolveJobPaths(syncResp.AssignedJob); err != nil {
			slog.Error("Failed to resolve job paths", "error", err)
			return nil
		}
		
		go w.executeJob(syncResp.AssignedJob)
	}
	
	return nil
}

// resolveJobPaths converts relative paths to absolute NAS paths
func (w *Worker) resolveJobPaths(job *models.JobSpec) error {
	// Resolve input source
	inputSource := job.GetInputSource()
	if inputSource == "" {
		return fmt.Errorf("job has no input source specified")
	}
	
	resolvedInput := w.resolveNASPath(inputSource)
	job.SetInputSource(resolvedInput)
	
	slog.Debug("Resolved input path", "path", resolvedInput)
	
	// Verify input file exists
	if _, err := os.Stat(resolvedInput); os.IsNotExist(err) {
		return fmt.Errorf("input file does not exist: %s", resolvedInput)
	}
	
	// Resolve output base path if present
	if job.OutputBase != "" {
		job.OutputBase = w.resolveNASPath(job.OutputBase)
		slog.Debug("Resolved output base", "path", job.OutputBase)
	}
	
	// Resolve each output rendition path
	for i := range job.Outputs {
		job.Outputs[i].DestPath = w.resolveNASPath(job.Outputs[i].DestPath)
		slog.Debug("Resolved output path",
			"resolution", job.Outputs[i].Resolution,
			"path", job.Outputs[i].DestPath)
		
		// Ensure output directory exists
		if err := os.MkdirAll(job.Outputs[i].DestPath, 0755); err != nil {
			return fmt.Errorf("failed to create output directory %s: %w", job.Outputs[i].DestPath, err)
		}
	}
	
	return nil
}

// resolveNASPath prepends NAS mount path if the given path is relative
func (w *Worker) resolveNASPath(path string) string {
	if filepath.IsAbs(path) {
		return path
	}
	
	cleanPath := filepath.Clean(path)
	cleanPath = strings.TrimPrefix(cleanPath, "/")
	
	return filepath.Join(w.cfg.NasMountPath, cleanPath)
}

// executeJob runs the transcoding process
func (w *Worker) executeJob(job *models.JobSpec) {
	w.jobMutex.Lock()
	w.currentJob = job
	w.jobMutex.Unlock()
	
	defer func() {
		w.jobMutex.Lock()
		w.currentJob = nil
		w.cancelJob = nil
		w.jobMutex.Unlock()
	}()
	
	// Create cancellable context
	jobCtx, cancel := context.WithCancel(context.Background())
	w.cancelJob = cancel
	defer cancel()
	
	startTime := time.Now()
	
	// Progress channel
	progressCh := make(chan models.JobProgress, 10)
	
	// Start progress reporter
	progressDone := make(chan struct{})
	go w.reportProgress(jobCtx, job.JobID, progressCh, progressDone)
	
	// Execute transcoding
	err := w.transcoder.Execute(jobCtx, job, progressCh)
	
	// Signal progress reporter to stop
	close(progressCh)
	<-progressDone
	
	// Finalize job
	duration := time.Since(startTime)
	w.finalizeJob(job, err, duration)
}

// reportProgress sends periodic progress updates
func (w *Worker) reportProgress(ctx context.Context, jobID string, progressCh <-chan models.JobProgress, done chan<- struct{}) {
	defer close(done)
	
	var lastProgress models.JobProgress
	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()
	
	for {
		select {
		case progress, ok := <-progressCh:
			if !ok {
				return
			}
			lastProgress = progress
			
		case <-ticker.C:
			if lastProgress.Percent > 0 {
				updateCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
				
				payload := models.JobStatusPayload{
					WorkerID:   w.cfg.WorkerID,
					Status:     "PROCESSING",
					Progress:   lastProgress.Percent,
					CurrentFPS: int(lastProgress.FPS),
					ETASec:     lastProgress.ETA,
				}
				
				if err := w.client.UpdateJobStatus(updateCtx, jobID, payload); err != nil {
					slog.Warn("Failed to send progress update", "error", err)
				}
				cancel()
			}
			
		case <-ctx.Done():
			return
		}
	}
}

// finalizeJob reports completion or failure
func (w *Worker) finalizeJob(job *models.JobSpec, jobErr error, duration time.Duration) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	
	payload := models.JobResultPayload{
		Metrics: models.JobMetrics{
			TotalTimeMS: duration.Milliseconds(),
		},
	}
	
	if jobErr != nil {
		slog.Error("Job failed",
			"job_id", job.JobID,
			"error", jobErr,
			"duration_ms", duration.Milliseconds())
		payload.Status = "FAILED"
		payload.ErrorMsg = jobErr.Error()
	} else {
		slog.Info("Job completed",
			"job_id", job.JobID,
			"duration", duration.String(),
			"duration_ms", duration.Milliseconds())
		payload.Status = "COMPLETED"
		
		// Construct manifest URL
		if len(job.Outputs) > 0 {
			outputPath := job.Outputs[0].DestPath
			relativeOutputPath := strings.TrimPrefix(outputPath, w.cfg.NasMountPath)
			relativeOutputPath = strings.TrimPrefix(relativeOutputPath, "/")
			
			playlistName := job.GetMasterPlaylistName()
			payload.ManifestURL = fmt.Sprintf("/%s/%s", relativeOutputPath, playlistName)
			
			slog.Info("Generated manifest", "url", payload.ManifestURL)
		}
	}
	
	if err := w.client.FinalizeJob(ctx, job.JobID, payload); err != nil {
		slog.Error("Failed to finalize job", "job_id", job.JobID, "error", err)
	}
}

// shutdown gracefully stops the worker
func (w *Worker) shutdown() {
	// Cancel current job if any
	w.jobMutex.Lock()
	if w.cancelJob != nil {
		slog.Warn("Cancelling current job due to shutdown")
		w.cancelJob()
	}
	w.jobMutex.Unlock()
	
	// Signal all goroutines to stop
	close(w.shutdownCh)
	
	// Wait for goroutines to finish
	w.wg.Wait()
	
	slog.Info("Shutdown complete")
}
