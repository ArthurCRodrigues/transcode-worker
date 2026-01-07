package main

import (
    "context"
    "fmt"
    "log"
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
    cfg         *config.Config
    client      *client.OrchestratorClient
    monitor     *monitor.SystemMonitor
    transcoder  *transcoder.FFmpegTranscoder
    
    currentJob  *models.JobSpec
    jobMutex    sync.Mutex
    
    cancelJob   context.CancelFunc
    
    shutdownCh  chan struct{}
    wg          sync.WaitGroup
}

func main() {
    // Load configuration
    cfg, err := config.Load(".")
    if err != nil {
        log.Fatalf("Failed to load config: %v", err)
    }

    log.Printf("Starting transcode worker: %s", cfg.WorkerID)
    log.Printf("Orchestrator URL: %s", cfg.OrchestratorURL)
    log.Printf("NAS Mount Path: %s", cfg.NasMountPath)
    log.Printf("Temp Directory: %s", cfg.TempDir)

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

    // Handle graceful shutdown
    sigCh := make(chan os.Signal, 1)
    signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)   

    // Start background workers
    worker.wg.Add(2)
    go worker.heartbeatLoop()
    go worker.jobPollingLoop()

    // Wait for shutdown signal
    <-sigCh
    log.Println("Shutdown signal received, cleaning up...")
    
    worker.shutdown()
    
    log.Println("Worker stopped gracefully")
}

// heartbeatLoop sends periodic health updates to orchestrator
func (w *Worker) heartbeatLoop() {
    defer w.wg.Done()
    
    ticker := time.NewTicker(w.cfg.HeartbeatInterval)
    defer ticker.Stop()
    
    log.Printf("Starting heartbeat loop (interval: %v)", w.cfg.HeartbeatInterval)
    
    for {
        select {
        case <-ticker.C:
            if err := w.sendHeartbeat(); err != nil {
                log.Printf("Heartbeat failed: %v", err)
            }
            
        case <-w.shutdownCh:
            log.Println("Heartbeat loop stopping...")
            return
        }
    }
}

// sendHeartbeat collects stats and reports to orchestrator
func (w *Worker) sendHeartbeat() error {
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
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
        status = "BUSY" // System is under load from other processes
    }
    w.jobMutex.Unlock()
    
    payload := models.HeartbeatPayload{
        WorkerID:      w.cfg.WorkerID,
        Status:        status,
        HardwareStats: stats,
        CurrentJobID:  currentJobID,
    }
    
    return w.client.Heartbeat(ctx, payload)
}

// jobPollingLoop continuously requests new jobs when idle
func (w *Worker) jobPollingLoop() {
    defer w.wg.Done()
    
    ticker := time.NewTicker(5 * time.Second)
    defer ticker.Stop()
    
    log.Println("Starting job polling loop...")
    
    for {
        select {
        case <-ticker.C:
            // Only poll if we're idle
            w.jobMutex.Lock()
            isIdle := w.currentJob == nil
            w.jobMutex.Unlock()
            
            if !isIdle {
                continue
            }
            
            // Check if system has capacity
            ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
            stats, err := w.monitor.GetStats(ctx)
            cancel()
            
            if err != nil {
                log.Printf("Failed to get stats during polling: %v", err)
                continue
            }
            
            if stats.IsBusy {
                log.Println("System is busy, skipping job request")
                continue
            }
            
            // Request a job
            if err := w.requestAndExecuteJob(); err != nil {
                if err.Error() != "no jobs available" {
                    log.Printf("Job execution error: %v", err)
                }
            }
            
        case <-w.shutdownCh:
            log.Println("Job polling loop stopping...")
            return
        }
    }
}

// requestAndExecuteJob requests a job from orchestrator and executes it
func (w *Worker) requestAndExecuteJob() error {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    // Get capabilities
    caps, err := w.monitor.GetCapabilities(ctx)
    if err != nil {
        return fmt.Errorf("failed to get capabilities: %w", err)
    }
    
    // Request job
    jobReq := models.JobRequestPayload{
        WorkerID:     w.cfg.WorkerID,
        Capabilities: caps,
    }
    
    job, err := w.client.RequestJob(ctx, jobReq)
    if err != nil {
        return err
    }
    
    if job == nil {
        return fmt.Errorf("no jobs available")
    }
    
    log.Printf("Received job: %s (Movie: %s)", job.JobID, job.MovieID)
    
    // Resolve NAS paths before execution
    if err := w.resolveJobPaths(job); err != nil {
        return fmt.Errorf("failed to resolve job paths: %w", err)
    }
    
    // Log job details
    log.Printf("Input: %s, Outputs: %d renditions", job.GetInputSource(), len(job.Outputs))
    
    // Execute the job
    w.executeJob(job)
    
    return nil
}

// resolveJobPaths converts relative paths from orchestrator to absolute NAS paths
func (w *Worker) resolveJobPaths(job *models.JobSpec) error {
    // Resolve input source path (handle both formats)
    inputSource := job.GetInputSource()
    if inputSource == "" {
        return fmt.Errorf("job has no input source specified")
    }
    
    resolvedInput := w.resolveNASPath(inputSource)
    
    // Update both fields to ensure compatibility
    if job.Input.SourceURL != "" {
        job.Input.SourceURL = resolvedInput
    }
    if job.InputSource != "" {
        job.InputSource = resolvedInput
    }
    // If neither was set, set InputSource
    if job.Input.SourceURL == "" && job.InputSource == "" {
        job.InputSource = resolvedInput
    }
    
    log.Printf("Resolved input path: %s", resolvedInput)
    
    // Verify input file exists
    if _, err := os.Stat(resolvedInput); os.IsNotExist(err) {
        return fmt.Errorf("input file does not exist: %s", resolvedInput)
    }
    
    // Resolve output base path if present
    if job.OutputBase != "" {
        job.OutputBase = w.resolveNASPath(job.OutputBase)
        log.Printf("Resolved output base: %s", job.OutputBase)
    }
    
    // Resolve each output rendition path
    for i := range job.Outputs {
        job.Outputs[i].DestPath = w.resolveNASPath(job.Outputs[i].DestPath)
        log.Printf("Resolved output [%s]: %s", job.Outputs[i].Resolution, job.Outputs[i].DestPath)
        
        // Ensure output directory exists
        if err := os.MkdirAll(job.Outputs[i].DestPath, 0755); err != nil {
            return fmt.Errorf("failed to create output directory %s: %w", job.Outputs[i].DestPath, err)
        }
    }
    
    return nil
}

// resolveNASPath prepends NAS mount path if the given path is relative
func (w *Worker) resolveNASPath(path string) string {
    // If path is already absolute, return as-is
    if filepath.IsAbs(path) {
        return path
    }
    
    // Clean the path to normalize slashes
    cleanPath := filepath.Clean(path)
    
    // Remove leading slash if present (for paths like "/raw/movie.mp4")
    cleanPath = strings.TrimPrefix(cleanPath, "/")
    
    // Join with NAS mount path
    absolutePath := filepath.Join(w.cfg.NasMountPath, cleanPath)
    
    return absolutePath
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
    
    // Create cancellable context for this job
    jobCtx, cancel := context.WithCancel(context.Background())
    w.cancelJob = cancel
    defer cancel()
    
    startTime := time.Now()
    
    // Progress channel
    progressCh := make(chan models.JobProgress, 10)
    
    // Start progress reporter goroutine
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

// reportProgress sends periodic progress updates to orchestrator
func (w *Worker) reportProgress(ctx context.Context, jobID string, progressCh <-chan models.JobProgress, done chan<- struct{}) {
    defer close(done)
    
    var lastProgress models.JobProgress
    ticker := time.NewTicker(2 * time.Second)
    defer ticker.Stop()
    
    for {
        select {
        case progress, ok := <-progressCh:
            if !ok {
                // Channel closed, transcoding finished
                return
            }
            lastProgress = progress
            
        case <-ticker.C:
            // Send periodic update
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
                    log.Printf("Failed to send progress update: %v", err)
                }
                cancel()
            }
            
        case <-ctx.Done():
            return
        }
    }
}

// finalizeJob reports completion or failure to orchestrator
func (w *Worker) finalizeJob(job *models.JobSpec, jobErr error, duration time.Duration) {
    ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
    defer cancel()
    
    payload := models.JobResultPayload{}
    payload.Metrics.TotalTimeMS = duration.Milliseconds()
    
    if jobErr != nil {
        log.Printf("Job %s FAILED: %v", job.JobID, jobErr)
        payload.Status = "FAILED"
        payload.ErrorMsg = jobErr.Error()
    } else {
        log.Printf("Job %s COMPLETED in %v", job.JobID, duration)
        payload.Status = "COMPLETED"
        
        // Construct manifest URL based on output paths
        // Use the first output's dest_path as the base for the manifest
        if len(job.Outputs) > 0 {
            outputPath := job.Outputs[0].DestPath
            // Convert absolute NAS path back to relative path for URL
            relativeOutputPath := strings.TrimPrefix(outputPath, w.cfg.NasMountPath)
            relativeOutputPath = strings.TrimPrefix(relativeOutputPath, "/")
            
            // Use the playlist name from job settings
            playlistName := job.GetMasterPlaylistName()
            payload.ManifestURL = fmt.Sprintf("/%s/%s", relativeOutputPath, playlistName)
            
            log.Printf("Manifest URL: %s", payload.ManifestURL)
        } else if job.OutputBase != "" {
            // Fallback to OutputBase if available
            relativeOutputPath := strings.TrimPrefix(job.OutputBase, w.cfg.NasMountPath)
            relativeOutputPath = strings.TrimPrefix(relativeOutputPath, "/")
            playlistName := job.GetMasterPlaylistName()
            payload.ManifestURL = fmt.Sprintf("/%s/%s", relativeOutputPath, playlistName)
            
            log.Printf("Manifest URL: %s", payload.ManifestURL)
        }
    }
    
    if err := w.client.FinalizeJob(ctx, job.JobID, payload); err != nil {
        log.Printf("Failed to finalize job: %v", err)
    }
}

// shutdown gracefully stops the worker
func (w *Worker) shutdown() {
    // Cancel current job if any
    w.jobMutex.Lock()
    if w.cancelJob != nil {
        log.Println("Cancelling current job...")
        w.cancelJob()
    }
    w.jobMutex.Unlock()
    
    // Signal all goroutines to stop
    close(w.shutdownCh)
    
    // Wait for goroutines to finish
    w.wg.Wait()
    
    // Send final offline heartbeat
    ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
    defer cancel()
    
    stats, _ := w.monitor.GetStats(ctx)
    finalPayload := models.HeartbeatPayload{
        WorkerID:      w.cfg.WorkerID,
        Status:        "OFFLINE",
        HardwareStats: stats,
    }
    
    if err := w.client.Heartbeat(ctx, finalPayload); err != nil {
        log.Printf("Failed to send final heartbeat: %v", err)
    }
}