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
	JobID        string          `json:"job_id"`
	MovieID      string          `json:"movie_id,omitempty"`
	Input        InputSpec       `json:"input,omitempty"`        // New format
	InputSource  string          `json:"input_source,omitempty"` // Legacy format - Path to raw file on NAS
	OutputBase   string          `json:"output_base,omitempty"`  // Base directory for outputs
	Outputs      []OutputSpec    `json:"outputs"`                // Target renditions
	HLSSettings  *HLSSettings    `json:"hls_settings,omitempty"` // New format
	Profile      *EncodingProfile `json:"profile,omitempty"`      // Legacy format - Encoding settings
	Priority     int             `json:"priority,omitempty"`
	CreatedAt    time.Time       `json:"created_at,omitempty"`
}

// InputSpec defines the input source (new format)
type InputSpec struct {
	SourceURL string `json:"source_url"` // Path to raw file (relative or absolute)
	Format    string `json:"format"`     // e.g. "mkv", "mp4", "avi"
}

// OutputSpec defines a single output rendition (e.g., 1080p variant)
type OutputSpec struct {
	Resolution string `json:"resolution"` // e.g. "1080p", "720p", "480p"
	Bitrate    string `json:"bitrate"`    // e.g. "5000k", "3000k"
	Codec      string `json:"codec"`      // e.g. "h264_nvenc", "libx264"
	DestPath   string `json:"dest_path"`  // Final destination path on NAS
}

// HLSSettings contains HLS-specific parameters (new format)
type HLSSettings struct {
	MasterPlaylistName string `json:"master_playlist_name"` // e.g. "index.m3u8"
	SegmentTime        int    `json:"segment_time"`         // Seconds per segment
}

// EncodingProfile contains encoding parameters (legacy format)
type EncodingProfile struct {
	Preset             string `json:"preset,omitempty"`              // e.g. "fast", "medium", "slow"
	HLSSegmentDuration int    `json:"hls_segment_duration,omitempty"` // Seconds per segment
	AudioCodec         string `json:"audio_codec,omitempty"`         // e.g. "aac"
	AudioBitrate       string `json:"audio_bitrate,omitempty"`       // e.g. "128k"
}



// GetHLSSegmentDuration returns the HLS segment duration, handling both new and legacy formats
func (j *JobSpec) GetHLSSegmentDuration() int {
	if j.HLSSettings != nil && j.HLSSettings.SegmentTime > 0 {
		return j.HLSSettings.SegmentTime
	}
	if j.Profile != nil && j.Profile.HLSSegmentDuration > 0 {
		return j.Profile.HLSSegmentDuration
	}
	return 6 // default
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



// Helper methods for JobSpec

// GetInputSource returns the input source path, handling both formats
func (j *JobSpec) GetInputSource() string {
    if j.Input.SourceURL != "" {
        return j.Input.SourceURL
    }
    return j.InputSource
}

// SetInputSource sets the input source in both fields for compatibility
func (j *JobSpec) SetInputSource(path string) {
    j.InputSource = path
    if j.Input.SourceURL != "" || j.Input.Format != "" {
        j.Input.SourceURL = path
    }
}

// GetSegmentTime returns the HLS segment duration, handling both formats
func (j *JobSpec) GetSegmentTime() int {
    if j.HLSSettings != nil && j.HLSSettings.SegmentTime > 0 {
        return j.HLSSettings.SegmentTime
    }
    if j.Profile != nil && j.Profile.HLSSegmentDuration > 0 {
        return j.Profile.HLSSegmentDuration
    }
    return 6 // Default to 6 seconds
}

// GetAudioCodec returns the audio codec, handling both formats
func (j *JobSpec) GetAudioCodec() string {
    if j.Profile != nil && j.Profile.AudioCodec != "" {
        return j.Profile.AudioCodec
    }
    return "aac" // Default codec
}

// GetAudioBitrate returns the audio bitrate, handling both formats
func (j *JobSpec) GetAudioBitrate() string {
    if j.Profile != nil && j.Profile.AudioBitrate != "" {
        return j.Profile.AudioBitrate
    }
    return "128k" // Default bitrate
}

// GetMasterPlaylistName returns the master playlist filename
func (j *JobSpec) GetMasterPlaylistName() string {
    if j.HLSSettings != nil && j.HLSSettings.MasterPlaylistName != "" {
        return j.HLSSettings.MasterPlaylistName
    }
    return "index.m3u8" // Default name
}