package models

// ===== Worker Registration & Capabilities =====

// RegistrationPayload is sent once on startup to declare worker capabilities
type RegistrationPayload struct {
	WorkerID     string              `json:"worker_id"`
	Capabilities WorkerCapabilities  `json:"capabilities"`
}

type WorkerCapabilities struct {
	SupportedCodecs []string `json:"supported_codecs"` // e.g. ["h264_nvenc", "hevc_nvenc", "libx264"]
	HasGPU          bool     `json:"has_gpu"`
	GPUType         string   `json:"gpu_type,omitempty"` // e.g. "nvidia", "intel", "amd"
	MaxResolution   string   `json:"max_resolution,omitempty"` // e.g. "4k", "1080p"
}

// ===== Worker Sync (Bidirectional Heartbeat + Job Assignment) =====

// SyncPayload is sent periodically to sync state with orchestrator
type SyncPayload struct {
	WorkerID      string        `json:"worker_id"`
	Status        string        `json:"status"` // "IDLE", "BUSY"
	HardwareStats HardwareStats `json:"hardware_stats"`
	CurrentJobID  string        `json:"current_job_id,omitempty"`
}

type HardwareStats struct {
	CPUPercent float64 `json:"cpu_percent"` // 0.0 to 100.0
	RAMPercent float64 `json:"ram_percent"` // 0.0 to 100.0
	IsBusy     bool    `json:"is_busy"`     // Computed: CPU > 80% or RAM > 90%
}

// SyncResponse is the orchestrator's response to a sync request
type SyncResponse struct {
	Ack         bool     `json:"ack"`
	AssignedJob *JobSpec `json:"assigned_job,omitempty"` // Present only if IDLE and work exists
}

// ===== Job Specification =====

// JobSpec represents a transcoding job sent by the orchestrator
type JobSpec struct {
	JobID       string          `json:"job_id"`
	MovieID     string          `json:"movie_id,omitempty"`
	Input       InputSpec       `json:"input"`
	OutputBase  string          `json:"output_base,omitempty"`
	Outputs     []OutputSpec    `json:"outputs"`
	HLSSettings HLSSettingsSpec `json:"hls_settings"`
	AudioConfig AudioConfigSpec `json:"audio_config,omitempty"`
}

// InputSpec represents the input source
type InputSpec struct {
	SourceURL string `json:"source_url"` // Path to raw file (relative to NAS mount)
	Format    string `json:"format,omitempty"`
}

// OutputSpec defines a single output rendition
type OutputSpec struct {
	Resolution   string `json:"resolution"` // e.g. "1080p", "720p"
	Bitrate      string `json:"bitrate"`    // e.g. "5000k", "2500k"
	Codec        string `json:"codec"`      // e.g. "h264_nvenc", "libx264"
	DestPath     string `json:"dest_path"`  // Final destination (relative to NAS mount)
	AudioCodec   string `json:"audio_codec,omitempty"`   // Per-rendition override
	AudioBitrate string `json:"audio_bitrate,omitempty"` // Per-rendition override
}

// HLSSettingsSpec represents HLS-specific settings
type HLSSettingsSpec struct {
	MasterPlaylistName string `json:"master_playlist_name,omitempty"` // Default: "index.m3u8"
	SegmentTime        int    `json:"segment_time,omitempty"`         // Default: 6 seconds
}

// AudioConfigSpec represents global audio encoding settings
type AudioConfigSpec struct {
	Codec   string `json:"codec,omitempty"`   // Default: "aac"
	Bitrate string `json:"bitrate,omitempty"` // Default: "128k"
}

// ===== JobSpec Helper Methods =====

// GetInputSource returns the input source path
func (j *JobSpec) GetInputSource() string {
	return j.Input.SourceURL
}

// SetInputSource sets the input source
func (j *JobSpec) SetInputSource(path string) {
	j.Input.SourceURL = path
}

// GetSegmentTime returns the HLS segment duration
func (j *JobSpec) GetSegmentTime() int {
	if j.HLSSettings.SegmentTime > 0 {
		return j.HLSSettings.SegmentTime
	}
	return 6 // Default
}

// GetAudioCodec returns the audio codec for a specific output
func (j *JobSpec) GetAudioCodec(output *OutputSpec) string {
	if output != nil && output.AudioCodec != "" {
		return output.AudioCodec
	}
	if j.AudioConfig.Codec != "" {
		return j.AudioConfig.Codec
	}
	return "aac" // Default
}

// GetAudioBitrate returns the audio bitrate for a specific output
func (j *JobSpec) GetAudioBitrate(output *OutputSpec) string {
	if output != nil && output.AudioBitrate != "" {
		return output.AudioBitrate
	}
	if j.AudioConfig.Bitrate != "" {
		return j.AudioConfig.Bitrate
	}
	return "128k" // Default
}

// GetMasterPlaylistName returns the master playlist filename
func (j *JobSpec) GetMasterPlaylistName() string {
	if j.HLSSettings.MasterPlaylistName != "" {
		return j.HLSSettings.MasterPlaylistName
	}
	return "index.m3u8" // Default
}

// ===== Job Progress & Status Updates =====

// JobStatusPayload is sent periodically during transcoding
type JobStatusPayload struct {
	WorkerID   string  `json:"worker_id"`
	Status     string  `json:"status"` // "PROCESSING"
	Progress   float64 `json:"progress,omitempty"`
	CurrentFPS int     `json:"current_fps,omitempty"`
	ETASec     int     `json:"eta_sec,omitempty"`
}

// JobProgress represents real-time progress during transcoding
type JobProgress struct {
	Percent float64 `json:"percent"`
	FPS     float64 `json:"fps"`
	ETA     int     `json:"eta"`
}

// ===== Job Completion =====

// JobResultPayload is sent when a job completes or fails
type JobResultPayload struct {
	Status      string     `json:"status"` // "COMPLETED" or "FAILED"
	ManifestURL string     `json:"manifest_url,omitempty"`
	ErrorMsg    string     `json:"error_msg,omitempty"`
	Metrics     JobMetrics `json:"metrics,omitempty"`
}

type JobMetrics struct {
	TotalTimeMS int64 `json:"total_time_ms"`
}
