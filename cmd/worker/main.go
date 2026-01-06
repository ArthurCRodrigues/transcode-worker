package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"
	"transcode-worker/internal/config"
	"transcode-worker/internal/heartbeat"
	"transcode-worker/internal/transcoder"
)

func main() {
	// 1. Load Configuration
	// It looks for config.yml in the root directory.
	cfg, err := config.LoadConfig("config.yml")
	if err != nil {
		log.Fatalf("Failed to load config: %v", err)
	}

	log.Printf("Starting Transcode Worker: %s", cfg.WorkerID)

	// 2. Initialize the Transcoding Engine
	// This performs the initial FFmpeg path lookup and hardware probing.
	engine, err := transcoder.NewEngine(
		true,
		0, // Default local temp dir
	)
    if err != nil {
        log.Fatalf("Failed to initialize transcoder engine: %v", err)
    }

	// 3. Setup Context for Graceful Shutdown
	// We catch SIGINT (Ctrl+C) and SIGTERM (OS shutdown).
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, os.Interrupt, syscall.SIGTERM)

	// 4. Start Heartbeat Service
	// This service handles the initial POST /v1/workers registration
	// and subsequent telemetry updates.
	hb := heartbeat.New(cfg.OrchestratorURL, cfg.HeartbeatSec, cfg.WorkerID, engine)
	hb.Start(ctx)

	log.Println("Worker is online and signaling to orchestrator.")

	// 5. Block until shutdown signal
	<-stop
	log.Println("Shutting down worker...")
	cancel()
}