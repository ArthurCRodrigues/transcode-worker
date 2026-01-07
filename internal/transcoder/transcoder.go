package transcoder

import (
    "bufio"
    "context"
    "fmt"
    "io"
    "log"
    "os"
    "os/exec"
    "path/filepath"
    "regexp"
    "strconv"
    "strings"
    //"time"

    "transcode-worker/pkg/models"
)

type FFmpegTranscoder struct {
    tempDir string
}

func NewTranscoder(tempDir string) *FFmpegTranscoder {
    return &FFmpegTranscoder{
        tempDir: tempDir,
    }
}

// Execute runs the transcoding job
func (t *FFmpegTranscoder) Execute(ctx context.Context, job *models.JobSpec, progressCh chan<- models.JobProgress) error {
    log.Printf("Starting transcoding job: %s", job.JobID)
    
    // Create job-specific temp directory
    jobTempDir := filepath.Join(t.tempDir, job.JobID)
    if err := os.MkdirAll(jobTempDir, 0755); err != nil {
        return fmt.Errorf("failed to create job temp dir: %w", err)
    }
    defer os.RemoveAll(jobTempDir) // Clean up temp files
    
    // Get media duration for progress calculation
    duration, err := t.getMediaDuration(job.GetInputSource())
    if err != nil {
        return fmt.Errorf("failed to get media duration: %w", err)
    }
    
    log.Printf("Media duration: %.2f seconds", duration)
    
    // Process each output rendition
    for i, output := range job.Outputs {
        log.Printf("Processing rendition %d/%d: %s (%s)", i+1, len(job.Outputs), output.Resolution, output.Bitrate)
        
        // Create temp output directory for this rendition
        renditionTempDir := filepath.Join(jobTempDir, fmt.Sprintf("%s_%s", output.Resolution, output.Bitrate))
        if err := os.MkdirAll(renditionTempDir, 0755); err != nil {
            return fmt.Errorf("failed to create rendition temp dir: %w", err)
        }
        
        // Transcode to temp directory
        if err := t.transcodeRendition(ctx, job, output, renditionTempDir, duration, progressCh); err != nil {
            return fmt.Errorf("failed to transcode %s: %w", output.Resolution, err)
        }
        
        // Copy files from temp to final destination
        if err := t.copyDirectory(renditionTempDir, output.DestPath); err != nil {
            return fmt.Errorf("failed to copy output files: %w", err)
        }
        
        log.Printf("Successfully completed rendition: %s", output.Resolution)
    }
    
    log.Printf("Transcoding job completed: %s", job.JobID)
    return nil
}

// transcodeRendition processes a single output rendition
func (t *FFmpegTranscoder) transcodeRendition(
    ctx context.Context,
    job *models.JobSpec,
    output models.OutputSpec,
    outputDir string,
    duration float64,
    progressCh chan<- models.JobProgress,
) error {
    // Get HLS settings
    segmentTime := job.GetSegmentTime()
    
    // Build FFmpeg command
    args := []string{
        "-i", job.GetInputSource(),
        "-c:v", output.Codec,
        "-b:v", output.Bitrate,
    }
    
    // Add resolution scaling if specified
    if output.Resolution != "" {
        scale := t.getScaleFilter(output.Resolution)
        if scale != "" {
            args = append(args, "-vf", scale)
        }
    }
    
    // Add audio encoding
    audioCodec := job.GetAudioCodec()
    audioBitrate := job.GetAudioBitrate()
    args = append(args,
        "-c:a", audioCodec,
        "-b:a", audioBitrate,
    )
    
    // Add HLS settings
    args = append(args,
        "-f", "hls",
        "-hls_time", fmt.Sprintf("%d", segmentTime),
        "-hls_playlist_type", "vod",
        "-hls_segment_filename", filepath.Join(outputDir, "segment_%03d.ts"),
        filepath.Join(outputDir, "index.m3u8"),
    )
    
    log.Printf("FFmpeg command: ffmpeg %s", strings.Join(args, " "))
    
    // Create FFmpeg command
    cmd := exec.CommandContext(ctx, "ffmpeg", args...)
    
    // Capture stderr for progress parsing
    stderr, err := cmd.StderrPipe()
    if err != nil {
        return fmt.Errorf("failed to get stderr pipe: %w", err)
    }
    
    // Start the command
    if err := cmd.Start(); err != nil {
        return fmt.Errorf("failed to start ffmpeg: %w", err)
    }
    
    // Parse progress from stderr
    go t.parseProgress(stderr, duration, progressCh)
    
    // Wait for completion
    if err := cmd.Wait(); err != nil {
        return fmt.Errorf("ffmpeg failed: %w", err)
    }
    
    return nil
}

// copyDirectory copies all files from src to dst, handling cross-device scenarios
func (t *FFmpegTranscoder) copyDirectory(src, dst string) error {
    log.Printf("Copying files from %s to %s", src, dst)
    
    // Ensure destination directory exists
    if err := os.MkdirAll(dst, 0755); err != nil {
        return fmt.Errorf("failed to create destination directory: %w", err)
    }
    
    // Read all files in source directory
    entries, err := os.ReadDir(src)
    if err != nil {
        return fmt.Errorf("failed to read source directory: %w", err)
    }
    
    // Copy each file
    for _, entry := range entries {
        if entry.IsDir() {
            continue // Skip subdirectories for now
        }
        
        srcFile := filepath.Join(src, entry.Name())
        dstFile := filepath.Join(dst, entry.Name())
        
        if err := t.copyFile(srcFile, dstFile); err != nil {
            return fmt.Errorf("failed to copy file %s: %w", entry.Name(), err)
        }
        
        log.Printf("Copied: %s", entry.Name())
    }
    
    log.Printf("Successfully copied %d files", len(entries))
    return nil
}

// copyFile copies a single file from src to dst
func (t *FFmpegTranscoder) copyFile(src, dst string) error {
    // Open source file
    srcFile, err := os.Open(src)
    if err != nil {
        return fmt.Errorf("failed to open source: %w", err)
    }
    defer srcFile.Close()
    
    // Create destination file
    dstFile, err := os.Create(dst)
    if err != nil {
        return fmt.Errorf("failed to create destination: %w", err)
    }
    defer dstFile.Close()
    
    // Copy contents
    if _, err := io.Copy(dstFile, srcFile); err != nil {
        return fmt.Errorf("failed to copy contents: %w", err)
    }
    
    // Sync to ensure data is written
    if err := dstFile.Sync(); err != nil {
        return fmt.Errorf("failed to sync destination: %w", err)
    }
    
    return nil
}

// getMediaDuration extracts total duration from media file using ffprobe
func (t *FFmpegTranscoder) getMediaDuration(inputPath string) (float64, error) {
    cmd := exec.Command("ffprobe",
        "-v", "error",
        "-show_entries", "format=duration",
        "-of", "default=noprint_wrappers=1:nokey=1",
        inputPath,
    )
    
    output, err := cmd.Output()
    if err != nil {
        return 0, fmt.Errorf("ffprobe failed: %w", err)
    }
    
    durationStr := strings.TrimSpace(string(output))
    duration, err := strconv.ParseFloat(durationStr, 64)
    if err != nil {
        return 0, fmt.Errorf("failed to parse duration: %w", err)
    }
    
    return duration, nil
}

// parseProgress monitors FFmpeg stderr and extracts progress information
func (t *FFmpegTranscoder) parseProgress(stderr io.Reader, totalDuration float64, progressCh chan<- models.JobProgress) {
    scanner := bufio.NewScanner(stderr)
    
    // Regex to extract time progress (e.g., "time=00:01:23.45")
    timeRegex := regexp.MustCompile(`time=(\d{2}):(\d{2}):(\d{2}\.\d{2})`)
    fpsRegex := regexp.MustCompile(`fps=\s*(\d+\.?\d*)`)
    
    for scanner.Scan() {
        line := scanner.Text()
        
        // Extract current time
        if matches := timeRegex.FindStringSubmatch(line); len(matches) == 4 {
            hours, _ := strconv.Atoi(matches[1])
            minutes, _ := strconv.Atoi(matches[2])
            seconds, _ := strconv.ParseFloat(matches[3], 64)
            
            currentTime := float64(hours*3600 + minutes*60) + seconds
            percent := (currentTime / totalDuration) * 100
            if percent > 100 {
                percent = 100
            }
            
            // Extract FPS
            var fps float64
            if fpsMatches := fpsRegex.FindStringSubmatch(line); len(fpsMatches) == 2 {
                fps, _ = strconv.ParseFloat(fpsMatches[1], 64)
            }
            
            // Calculate ETA
            var eta int
            if fps > 0 {
                remainingSeconds := totalDuration - currentTime
                eta = int(remainingSeconds / fps)
            }
            
            // Send progress update
            progress := models.JobProgress{
                Percent: percent,
                FPS:     fps,
                ETA:     eta,
            }
            
            select {
            case progressCh <- progress:
            default:
                // Channel full, skip this update
            }
        }
    }
}

// getScaleFilter returns FFmpeg scale filter for the given resolution
func (t *FFmpegTranscoder) getScaleFilter(resolution string) string {
    switch resolution {
    case "2160p", "4K":
        return "scale=-2:2160"
    case "1080p":
        return "scale=-2:1080"
    case "720p":
        return "scale=-2:720"
    case "480p":
        return "scale=-2:480"
    case "360p":
        return "scale=-2:360"
    default:
        return ""
    }
}