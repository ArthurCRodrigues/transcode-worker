package transcoder

import (
	"os/exec"
	"runtime"
	"strings"

	"github.com/shirou/gopsutil/v3/cpu"
	//"github.com/shirou/gopsutil/v3/host"
	"github.com/shirou/gopsutil/v3/mem"
	"transcode-worker/pkg/models"
)

// ProbeCapabilities checks FFmpeg for encoders.
func (e *Engine) ProbeCapabilities() {
	cmd := exec.Command(e.FFmpegPath, "-encoders")
	output, err := cmd.CombinedOutput()
	if err != nil {
		e.HasHWAccel = false
		e.bestCodec = CodecSoftware
		return
	}

	outStr := string(output)
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

// GetStaticSpecs gathers hardware info that doesn't change.
func (e *Engine) GetStaticSpecs() models.StaticHardware {
	info, _ := cpu.Info()
	model := "Unknown CPU"
	if len(info) > 0 {
		model = info[0].ModelName
	}

	accel := []string{}
	if e.HasHWAccel {
		accel = append(accel, e.bestCodec)
	}

	return models.StaticHardware{
		CPUModel:             model,
		TotalThreads:         runtime.NumCPU(),
		HardwareAcceleration: accel,
	}
}

// GetSystemHealth gathers real-time telemetry.
func (e *Engine) GetSystemHealth() models.SystemHealth {
	v, _ := mem.VirtualMemory()
	c, _ := cpu.Percent(0, false)
	
	cpuUsage := 0.0
	if len(c) > 0 {
		cpuUsage = c[0]
	}

	// Note: GPU usage and Temp require specialized tools (NVML/sensors)
	// For this first test, we focus on CPU and RAM.
	return models.SystemHealth{
		CPUUsage:     cpuUsage,
		RAMFreeBytes: v.Available,
		TempC:        0, // Placeholder
	}
}