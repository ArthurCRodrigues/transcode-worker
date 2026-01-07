package models

import "time"

// Payload for POST /heartbeat
type HeartbeatPayload struct {
	WorkerID      string        `json:"worker_id"`
	Status        string        `json:"status"` // "IDLE", "BUSY", "OFFLINE"
	HardwareStats HardwareStats `json:"hardware"`
	CurrentJobID  string        `json:"current_job_id,omitempty"`
}

type HardwareStats struct {
	// CPU usage percentage (0.0 to 100.0)
	CpuUsagePercent float64 `json:"cpu_usage_percent"`

	// Available RAM in Gigabytes
	RamAvailableGB float64 `json:"ram_available_gb"`

	// Computed flag: Is the system too busy to accept new work?
	// This is calculated by the Monitor based on thresholds (e.g. CPU > 80%).
	IsBusy bool `json:"is_busy"`
}

// Payload for POST /jobs/request
type JobRequestPayload struct {
	WorkerID     string   `json:"worker_id"`
	Capabilities []string `json:"capabilities"` // e.g. ["nvenc", "4k", "h265"]
}

// Payload for PATCH /jobs/{id}
type JobStatusPayload struct {
	WorkerID   string  `json:"worker_id"`
	Status     string  `json:"status"` // "PROCESSING"
	Progress   float64 `json:"progress"`
	CurrentFPS int     `json:"current_fps"`
	ETASec     int     `json:"eta_seconds"`
}

// Payload for POST /jobs/{id}/finalize
type JobResultPayload struct {
	Status      string `json:"status"` // "COMPLETED", "FAILED"
	ManifestURL string `json:"output_manifest_url,omitempty"`
	ErrorMsg    string `json:"error_message,omitempty"`
	Metrics     struct {
		TotalTimeMS int64 `json:"total_time_ms"`
		AvgFPS      int   `json:"avg_fps"`
	} `json:"metrics"`
}

// JobSpec represents a transcoding job received from the orchestrator
type JobSpec struct {
	JobID        string         `json:"job_id"`
	MovieID      string         `json:"movie_id"`
	InputSource  string         `json:"input_source"`  // Path to raw file on NAS
	OutputBase   string         `json:"output_base"`   // Base directory for outputs
	Outputs      []OutputSpec   `json:"outputs"`       // Target renditions
	Profile      EncodingProfile `json:"profile"`       // Encoding settings
	Priority     int            `json:"priority"`
	CreatedAt    time.Time      `json:"created_at"`
}

// OutputSpec defines a single output rendition (e.g., 1080p variant)
type OutputSpec struct {
	Resolution string `json:"resolution"` // e.g. "1080p", "720p", "480p"
	Bitrate    string `json:"bitrate"`    // e.g. "5000k", "3000k"
	Codec      string `json:"codec"`      // e.g. "h264_nvenc", "libx264"
	DestPath   string `json:"dest_path"`  // Final destination path on NAS
}

// EncodingProfile contains encoding parameters
type EncodingProfile struct {
	Preset       string `json:"preset"`        // e.g. "fast", "medium", "slow"
	HLSSegmentDuration int `json:"hls_segment_duration"` // Seconds per segment
	AudioCodec   string `json:"audio_codec"`   // e.g. "aac"
	AudioBitrate string `json:"audio_bitrate"` // e.g. "128k"
}

// JobProgress represents real-time progress during transcoding
type JobProgress struct {
	Percent float64 `json:"percent"`
	FPS     float64 `json:"fps"`
	ETA     int     `json:"eta_seconds"`
}

// TranscodeJob is used internally for job management (if needed)
type TranscodeJob struct {
	Spec      JobSpec
	Status    string
	StartTime time.Time
	EndTime   time.Time
	Error     error
}