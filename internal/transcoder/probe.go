package transcoder

import (
	"os/exec"
	"strings"
)

// Hardware capabilities constants
const (
	CodecNVENC       = "h264_nvenc"       // NVIDIA
	CodecVAAPI       = "h264_vaapi"       // Linux (Intel/AMD)
	CodecVideoToolbox = "h264_videotoolbox" // macOS
	CodecSoftware    = "libx264"          // CPU Fallback
)

// ProbeCapabilities checks the system for the best available hardware encoder.
func (e *Engine) ProbeCapabilities() {
	// Execute 'ffmpeg -encoders'
	cmd := exec.Command(e.FFmpegPath, "-encoders")
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.HasHWAccel = false
		return
	}

	outStr := string(output)

	// Priority Check: We check for the most powerful encoders first.
	if strings.Contains(outStr, "nvenc") {
		e.bestCodec = CodecNVENC
		e.HasHWAccel = true
	} else if strings.Contains(outStr, "vaapi") {
		e.bestCodec = CodecVAAPI
		e.HasHWAccel = true
	} else if strings.Contains(outStr, "videotoolbox") {
		e.bestCodec = CodecVideoToolbox
		e.HasHWAccel = true
	} else {
		e.bestCodec = CodecSoftware
		e.HasHWAccel = false
	}
}