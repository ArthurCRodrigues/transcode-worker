package transcoder

import (
	"fmt"
	"os/exec"
)

// Define constants for supported codecs to avoid "magic strings" in the code.
// These are used by probe.go to set the state and command.go to build the CLI args.
const (
	CodecNVENC         = "h264_nvenc"
	CodecVAAPI         = "h264_vaapi"
	CodecVideoToolbox  = "h264_videotoolbox"
	CodecSoftware      = "libx264"
)

// Engine represents the transcoding capabilities of the local device.
// Its state is populated by probe.go and its methods are called by executor.go.
type Engine struct {
	FFmpegPath string
	HasHWAccel bool
	bestCodec  string
	maxThreads int
}

// NewEngine initializes the transcoder headquarters.
// It finds the binary and then calls the probing logic defined in probe.go.
func NewEngine(allowHW bool, threads int) (*Engine, error) {
	// 1. Locate the FFmpeg binary on the system PATH.
	path, err := exec.LookPath("ffmpeg")
	if err != nil {
		return nil, fmt.Errorf("ffmpeg binary not found: %w", err)
	}

	// 2. Create the initial Engine instance.
	engine := &Engine{
		FFmpegPath: path,
		maxThreads: threads,
	}

	// 3. Perform hardware discovery.
	// This function is defined in probe.go
	if allowHW {
		engine.ProbeCapabilities()
	} else {
		engine.bestCodec = CodecSoftware
		engine.HasHWAccel = false
	}

	return engine, nil
}

// GetCodec is a helper used by command.go to select the right encoder string.
func (e *Engine) GetCodec() string {
	if e.bestCodec == "" {
		return CodecSoftware
	}
	return e.bestCodec
}