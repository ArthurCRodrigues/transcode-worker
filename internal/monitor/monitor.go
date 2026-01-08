package monitor

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strings"
	"sync"
	"time"

	"github.com/shirou/gopsutil/v3/cpu"
	"github.com/shirou/gopsutil/v3/mem"
	"transcode-worker/pkg/models"
)

type SystemMonitor struct {
	cachedCaps []string
	once       sync.Once
	ffmpegPath string
}

func NewSystemMonitor() *SystemMonitor {
	// Assume ffmpeg is in PATH. In a real deployment, 
	// you might want to configure the path via config.yml
	return &SystemMonitor{
		ffmpegPath: "ffmpeg",
	}
}

// GetCapabilities runs once to discover what this worker can do.
// It checks FFmpeg encoders, not just drivers, to ensure the software stack is ready.
func (m *SystemMonitor) GetCapabilities(ctx context.Context) ([]string, error) {
	var err error
	
	// We use sync.Once because hardware capabilities don't change at runtime.
	m.once.Do(func() {
		m.cachedCaps, err = m.detectFFmpegCapabilities(ctx)
	})

	if err != nil {
		return nil, err
	}
	return m.cachedCaps, nil
}

// GetStats gathers real-time CPU and RAM usage.
func (m *SystemMonitor) GetStats(ctx context.Context) (models.HardwareStats, error) {
	stats := models.HardwareStats{}

	// 1. Get Memory Stats
	v, err := mem.VirtualMemoryWithContext(ctx)
	if err != nil {
		return stats, fmt.Errorf("failed to get mem stats: %w", err)
	}
	// UsedPercent returns percentage of memory used (0.0 to 100.0)
	stats.RAMPercent = v.UsedPercent

	// 2. Get CPU Percent (over the last 500ms)
	// Passing 0 as duration returns immediate value (gauge), but a small interval is more accurate.
	cpuPct, err := cpu.PercentWithContext(ctx, 500*time.Millisecond, false)
	if err != nil {
		return stats, fmt.Errorf("failed to get cpu stats: %w", err)
	}
	
	if len(cpuPct) > 0 {
		stats.CPUPercent = cpuPct[0]
	}

	// 3. Busy Logic (Optional Domain Logic)
	// If CPU > 80% or RAM > 90%, mark as busy so the scheduler skips us.
	stats.IsBusy = stats.CPUPercent > 80.0 || stats.RAMPercent > 90.0

	return stats, nil
}

// detectFFmpegCapabilities asks FFmpeg what it supports.
// This is safer than checking drivers because it proves FFmpeg can actually SEE the hardware.
func (m *SystemMonitor) detectFFmpegCapabilities(ctx context.Context) ([]string, error) {
	// Command: ffmpeg -hide_banner -encoders
	cmd := exec.CommandContext(ctx, m.ffmpegPath, "-hide_banner", "-encoders")
	var out bytes.Buffer
	cmd.Stdout = &out
	
	if err := cmd.Run(); err != nil {
		return nil, fmt.Errorf("ffmpeg check failed: %w", err)
	}

	output := out.String()
	var caps []string

	// Basic Resolution support (assumed true for any modern CPU, but good to flag)
	caps = append(caps, "1080p", "720p")

	// Detect HW Accelerators
	if strings.Contains(output, "h264_nvenc") || strings.Contains(output, "hevc_nvenc") {
		caps = append(caps, "nvenc", "h264_nvenc")
	}
	if strings.Contains(output, "h264_qsv") {
		caps = append(caps, "qsv", "quicksync")
	}
	if strings.Contains(output, "h264_vaapi") {
		caps = append(caps, "vaapi")
	}
	if strings.Contains(output, "h264_v4l2m2m") {
		caps = append(caps, "rpi", "v4l2m2m") // Raspberry Pi hardware encoding
	}

	// Heuristic for 4K support
	// If we have NVENC or QSV, we assume 4K is possible.
	// A strictly software-only worker might struggle with 4K.
	for _, c := range caps {
		if c == "nvenc" || c == "qsv" || c == "vaapi" {
			caps = append(caps, "4k")
			break
		}
	}

	return caps, nil
}