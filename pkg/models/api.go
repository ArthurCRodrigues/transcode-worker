package models

// --- Registration & Worker Management ---

// WorkerRegistration represents the initial handshake payload sent by the worker.
// Used in [POST] /v1/workers
type WorkerRegistration struct {
	ID          string         `json:"id"`
	BaseURL     string         `json:"base_url"`
	StaticSpecs StaticHardware `json:"static_specs"`
}

// StaticHardware defines immutable specs reported once at startup to help the 
// orchestrator make scheduling decisions based on raw power.
type StaticHardware struct {
	CPUModel             string   `json:"cpu_model"`
	TotalThreads         int      `json:"total_threads"`
	HardwareAcceleration []string `json:"hardware_acceleration"` // e.g., ["nvenc", "vaapi", "cuda"]
}

// WorkerStatusUpdate handles explicit state changes, such as the "Death Note".
// Used in [PATCH] /v1/workers/:id
type WorkerStatusUpdate struct {
	Status string `json:"status"` // e.g., "OFFLINE", "MAINTENANCE"
	Reason string `json:"reason,omitempty"`
}

// --- Job & Configuration Management ---

// TranscodeJob defines the work to be performed. Note that it does NOT contain
// the output path, as the worker manages its own local filesystem.
// Used in [POST] /v1/jobs
type TranscodeJob struct {
	JobID         string    `json:"job_id"`
	SourcePath    string    `json:"source_path"`
	Configuration JobConfig `json:"configuration"`
}

// JobConfig contains specific FFmpeg parameters. This is used for both 
// initial starts and time-skips (seeks).
type JobConfig struct {
	SeekTime      string `json:"seek_time"`      // Format: "HH:MM:SS"
	BitrateBps    int    `json:"bitrate_bps"`    // Target bitrate in bits per second
	Encoder       string `json:"encoder"`        // Specifically requested codec
	ForceRemux    bool   `json:"force_remux"`    // Try to copy stream without re-encoding
	Resolution    string `json:"resolution"`     // e.g., "1080p", "720p"
}

// --- Heartbeat & Telemetry ---

// Heartbeat represents the 10s telemetry pulse sent to the orchestrator.
// Used in [POST] /v1/workers/:id/heartbeats
type Heartbeat struct {
	Status     string         `json:"status"` // IDLE, BUSY, STRESSED, ERROR
	Telemetry  SystemHealth   `json:"telemetry"`
	JobContext *ActiveContext `json:"job_context,omitempty"`
	Error      *WorkerError   `json:"error_context,omitempty"`
}

// SystemHealth captures real-time hardware metrics gathered by gopsutil and nvml.
type SystemHealth struct {
	CPUUsage     float64 `json:"cpu_usage"`      // Percentage
	GPUUsage     float64 `json:"gpu_usage"`      // Percentage
	RAMFreeBytes uint64  `json:"ram_free_bytes"`
	TempC        float64 `json:"temp_c"`         // Celsius
}

// ActiveContext provides progress data for the currently running job.
type ActiveContext struct {
	ActiveJobID   string  `json:"active_job_id"`
	Progress      float64 `json:"progress"`        // 0-100%
	Speed         string  `json:"speed"`           // e.g., "1.5x"
	LastSegmentID int     `json:"last_segment_id"` // Sequence counter
}

// WorkerError provides details for the orchestrator when a job or worker fails.
type WorkerError struct {
	Code    string `json:"code"`
	Message string `json:"message"`
}
