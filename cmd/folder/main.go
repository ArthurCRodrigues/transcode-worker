func main() {
    cfg := config.LoadConfig("config.yml")
    
    // Create a context that we can cancel later
    ctx, cancel := context.WithCancel(context.Background())
    defer cancel()

    hb := heartbeat.New(cfg.OrchestratorURL, cfg.HeartbeatSec, cfg.WorkerID)
    hb.Start(ctx)

    // Keep the main process alive (otherwise the app exits immediately)
    select {} 
}