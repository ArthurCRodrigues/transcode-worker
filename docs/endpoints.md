# Complete Endpoint Map for the Orchestrator-Worker Communication 
To understand the inter-service lifecycle, see [communication.md](communication.md) and [worker_orchestration.md](worker_orchestration.md)

## 1. Outbound: Worker &rarr; Orchestrator
The worker is a "talkative" agent. It initiates these calls to keep the Orchestrator informed of its status and progress. 
`POST /v1/workers` (Registration)
- **Frequency:** Handshake performed at once on at worker startup.
- **Purpose:** Keeps the worker "Alive" in the orchestrator's worker pool and shares hardware telemetry.
- **Payload:**
  ```json
   {
    "id": "string",            // "fedora-desktop-01"
    "static_specs": {
      "cpu_model": "string",
      "total_threads": 16,
      "hardware_acceleration": ["nvenc", "vaapi", "cuda"] // List of APIs
    }
  ```
`POST /v1/workers/:id/heartbeats` (telemetry)
- **Frequency:** Every 10-15 seconds.
- **Purpose:** Real time monitoring.
- **Payload:**
  ```json
  {
  "status": "string",        // IDLE, BUSY, STRESSED, ERROR
  "telemetry": {
    "cpu_usage": 45.2,       // %
    "gpu_usage": 12.0,       // %
    "ram_free_bytes": 1024,  // Real-time memory pressure
    "temp_c": 65.0           // Thermal health
  },
  "job_context": {
    "active_job_id": "string", 
    "progress": 15.4,        // % of movie completed
    "speed": "1.8x",         // Transcode speed vs play time
    "last_segment_id": 402   // Sequence number of the latest .ts file
  }
  ```
`PATCH /v1/workers/:id` ("Death-Note")
- **Frequency:** When `SIGKILL` is detected or on runtime errors.
- **Purpose:** Immediate state update for graceful shutdown. 
- **Payload:**
  ```json
   {
    "status": "OFFLINE",
    "reason": "SHUTDOWN"
  }
  ```
## 2. Inbound: Orchestrator &rarr; Worker
These endpoints live on the worker. They allow the Orchestrator to control the transcoding process.
`POST /v1/workers/:id/heartbeats` (controller)
- **Frequency:** When there's a request to play a video.
- **Purpose:** Creates a new transcoding context or updates an existing one (Idempotent!!).
- **Payload:**
  ```json
  {
  "job_id": "string",       // Unique session UUID
  "source_path": "string",  // Absolute path to MKV/MP4 on the NAS SSD
  "output_path": "string",  // Local folder for HLS segments
  "configuration": {
    "seek_time": "string",  // "HH:MM:SS" (Crucial for Skips/Seeks)
    "bitrate_bps": 5000000, // Target bitrate in bits per second
    "encoder": "string",    // Specifically requested codec (e.g., "h264_nvenc")
    "force_remux": boolean  // If true, attempts "Stream Copy" to save CPU
  }
  ```
- **Behavior:**
  1. If `job_id` is new: Spawn FFmpeg
  2. If `job_id`is running: Kill existing PID &rarr; Clear temp folder -> Respawn at `seek_time`

`DELETE /v1/jobs/:id` (termination)
- **Frequency:** When there's a request stop/quit playing a video.
- **Purpose:** Gradecully stops a job and cleans up resources.
- **Behavior:** Sends `SIGTERM`to the FFmpeg process and deletes the local segment directory.


