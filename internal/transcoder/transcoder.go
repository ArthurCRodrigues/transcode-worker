package transcoder

import (
	"bufio"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"transcode-worker/pkg/models"
)

type FFmpegTranscoder struct {
	binPath    string
	probePath  string
	tempDir    string // Base temp dir
}

func NewTranscoder(tempDir string) *FFmpegTranscoder {
	return &FFmpegTranscoder{
		binPath:   "ffmpeg",
		probePath: "ffprobe",
		tempDir:   tempDir,
	}
}

// Execute handles the full lifecycle of a transcoding job
func (t *FFmpegTranscoder) Execute(ctx context.Context, job *models.JobSpec, progressCh chan<- models.JobProgress) error {
	// 1. Create a specific temp folder for this job
	jobTempDir := filepath.Join(t.tempDir, job.JobID)
	if err := os.MkdirAll(jobTempDir, 0755); err != nil {
		return fmt.Errorf("failed to create temp dir: %w", err)
	}
	// Cleanup on finish (optional, depending on if you want to debug failed jobs)
	defer os.RemoveAll(jobTempDir)

	// 2. Probe Input to get Duration (needed for progress calculation)
	durationSec, err := t.probeDuration(ctx, job.InputSource)
	if err != nil {
		return fmt.Errorf("probe failed: %w", err)
	}

	// 3. Construct the FFmpeg Command
	args := t.buildArgs(job, jobTempDir)
	
	cmd := exec.CommandContext(ctx, t.binPath, args...)
	
	// 4. Capture Stderr for progress parsing
	stderrPipe, err := cmd.StderrPipe()
	if err != nil {
		return fmt.Errorf("failed to get stderr pipe: %w", err)
	}

	if err := cmd.Start(); err != nil {
		return fmt.Errorf("failed to start ffmpeg: %w", err)
	}

	// 5. Monitor Progress in a Goroutine
	doneCh := make(chan error, 1)
	go func() {
		scanner := bufio.NewScanner(stderrPipe)
		// Regex to catch "time=00:00:15.45"
		reTime := regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2}\.\d+)`)
		// Regex to catch "fps= 24"
		reFPS := regexp.MustCompile(`fps=\s*(\d+)`)

		for scanner.Scan() {
			line := scanner.Text()
			
			// Parse Time for Percentage
			matches := reTime.FindStringSubmatch(line)
			if len(matches) == 4 {
				h, _ := strconv.Atoi(matches[1])
				m, _ := strconv.Atoi(matches[2])
				s, _ := strconv.ParseFloat(matches[3], 64)
				currentSec := float64(h*3600 + m*60) + s
				
				pct := (currentSec / durationSec) * 100
				if pct > 100 { pct = 100 }

				// Parse FPS (Optional, simple extraction)
				fpsVal := 0.0
				fpsMatch := reFPS.FindStringSubmatch(line)
				if len(fpsMatch) > 1 {
					if v, err := strconv.ParseFloat(fpsMatch[1], 64); err == nil {
						fpsVal = v
					}
				}

				// Send Update
				select {
				case progressCh <- models.JobProgress{Percent: pct, FPS: fpsVal}:
				default:
					// Drop update if channel is blocked (control plane is slow)
				}
			}
		}
		doneCh <- scanner.Err()
	}()

	// 6. Wait for Finish
	if err := cmd.Wait(); err != nil {
		return fmt.Errorf("ffmpeg process failed: %w", err)
	}
	
	// 7. Move Files from Temp to Final Destination (NAS)
	// Note: In a real HLS scenario, we need to move the .m3u8 AND the .ts segments.
	for _, output := range job.Outputs {
		// Identify the subfolder in temp for this specific output variant
		// (This logic matches the buildArgs logic below)
		variantName := fmt.Sprintf("%s_%s", output.Resolution, output.Bitrate)
		srcDir := filepath.Join(jobTempDir, variantName)
		
		// Ensure destination exists
		if err := os.MkdirAll(output.DestPath, 0755); err != nil {
			return fmt.Errorf("failed to create nas dir: %w", err)
		}

		// Move all files
		files, _ := os.ReadDir(srcDir)
		for _, f := range files {
			oldPath := filepath.Join(srcDir, f.Name())
			newPath := filepath.Join(output.DestPath, f.Name())
			if err := os.Rename(oldPath, newPath); err != nil {
				return fmt.Errorf("failed to move file %s: %w", f.Name(), err)
			}
		}
	}

	return nil
}

// probeDuration uses ffprobe to get media duration in seconds
func (t *FFmpegTranscoder) probeDuration(ctx context.Context, path string) (float64, error) {
	args := []string{
		"-v", "error",
		"-show_entries", "format=duration",
		"-of", "json",
		path,
	}
	cmd := exec.CommandContext(ctx, t.probePath, args...)
	output, err := cmd.Output()
	if err != nil {
		return 0, err
	}

	type ProbeResult struct {
		Format struct {
			Duration string `json:"duration"`
		} `json:"format"`
	}
	var res ProbeResult
	if err := json.Unmarshal(output, &res); err != nil {
		return 0, err
	}

	return strconv.ParseFloat(res.Format.Duration, 64)
}

// buildArgs constructs the complex ffmpeg command
func (t *FFmpegTranscoder) buildArgs(job *models.JobSpec, jobTempDir string) []string {
	// Basic input args
	args := []string{
		"-y",                 // Overwrite output
		"-i", job.InputSource, // Input file
		"-hide_banner",
	}

	// For every output definition in the job, we add mapping arguments.
	// We are generating HLS directly.
	for _, out := range job.Outputs {
		// e.g. variant folder: /tmp/job123/1080p_5000k/
		variantName := fmt.Sprintf("%s_%s", out.Resolution, out.Bitrate)
		variantDir := filepath.Join(jobTempDir, variantName)
		_ = os.MkdirAll(variantDir, 0755)

		// HLS segment filename pattern
		segmentPath := filepath.Join(variantDir, "segment_%03d.ts")
		playlistPath := filepath.Join(variantDir, "index.m3u8")

		// Scaling filter
		// "scale=-2:1080" keeps aspect ratio, ensures width is divisible by 2 (requirement for some encoders)
		height := strings.TrimSuffix(out.Resolution, "p")
		scaleFilter := fmt.Sprintf("scale=-2:%s", height)

		args = append(args,
			"-vf", scaleFilter,
			"-c:v", out.Codec,      // e.g. h264_nvenc
			"-b:v", out.Bitrate,    // e.g. 5000k
			"-c:a", "aac",          // Audio is usually AAC for HLS
			"-b:a", "128k",
			"-f", "hls",
			"-hls_time", "6",       // Standard segment duration
			"-hls_list_size", "0",  // Keep all segments in playlist (VOD mode)
			"-hls_segment_filename", segmentPath,
			playlistPath,
		)
	}

	return args
}