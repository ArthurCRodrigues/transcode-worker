# transcode-worker

This project provides a worker service managing heavy transcoding jobs. It is meant for allowing low-power weak media servers to keep providing high quality data by delegating the heavy video transcoding job to available powerful devices across a worker registry.

## Architecture Overview

The system consists of two main components:

### Control Plane (Orchestrator)
- Accepts movie uploads and stores raw files
- Stores movie metadata in database
- Creates transcoding jobs
- Assigns jobs to available workers
- Tracks job states and progress
- Serves HLS files to clients
- Exposes playback API

### Data Plane (Transcode Workers)
- Register with orchestrator on startup
- Send periodic heartbeats with hardware stats (CPU, RAM, GPU capabilities)
- Poll for transcoding jobs when idle
- Execute FFmpeg transcoding (read raw media → generate HLS output)
- Report progress and completion status
- Handle graceful shutdown and job cancellation

## Features

✅ **Automatic Registration**: Workers self-register with the orchestrator on startup  
✅ **Health Monitoring**: Periodic heartbeats report CPU/RAM usage and current status  
✅ **Hardware Capability Detection**: Auto-detects NVENC, QSV, VAAPI, and other accelerators  
✅ **Job Polling**: Continuously polls for new transcoding jobs when idle  
✅ **FFmpeg Integration**: Executes transcoding with progress tracking  
✅ **HLS Output**: Generates adaptive bitrate HLS segments  
✅ **Progress Reporting**: Real-time updates to orchestrator (percentage, FPS, ETA)  
✅ **Graceful Shutdown**: Cancels running jobs and notifies orchestrator  
✅ **NAS Integration**: Reads raw files and writes outputs to shared storage  
✅ **Fault Tolerance**: Jobs can be re-run on another worker if one fails  

## Prerequisites

- **Go 1.21+** for building
- **FFmpeg** with desired encoders (h264_nvenc, libx264, etc.)
- **FFprobe** for media inspection
- **Mounted NAS storage** for accessing raw files and storing outputs

## Installation

1. Clone the repository:
```bash
git clone <repository-url>
cd transcode-worker
```

2. Install dependencies:
```bash
go mod download
```

3. Build the worker:
```bash
go build -o bin/worker cmd/worker/main.go
```

Or use the Makefile:
```bash
make build
```

## Configuration

Copy the example configuration:
```bash
cp config-example.yml config.yml
```

Edit `config.yml` with your settings:

```yaml
# [REQUIRED] The orchestrator URL
orchestrator_url: "http://192.168.1.100:8080"

# [OPTIONAL] Worker ID (defaults to hostname)
worker_id: ""

# [REQUIRED] NAS mount path
nas_mount_path: "/mnt/nas"

# [OPTIONAL] Temp directory for transcoding
temp_dir: "/tmp/transcode-worker"

# [OPTIONAL] Heartbeat interval
heartbeat_interval: 10s

# [OPTIONAL] Max concurrent jobs
max_concurrent_jobs: 1

# [OPTIONAL] Log level
log_level: "info"
```

### Environment Variables

You can also configure via environment variables (they override config file):

```bash
export WORKER_ORCHESTRATOR_URL="http://192.168.1.100:8080"
export WORKER_NAS_MOUNT_PATH="/mnt/nas"
export WORKER_TEMP_DIR="/tmp/transcode"
export WORKER_HEARTBEAT_INTERVAL="15s"
```

## Usage

### Running the Worker

```bash
./bin/worker
```

Or with custom config path:
```bash
./bin/worker -config /path/to/config.yml
```

### Running as a Service

Create a systemd service file `/etc/systemd/system/transcode-worker.service`:

```ini
[Unit]
Description=Transcode Worker
After=network.target

[Service]
Type=simple
User=your-user
WorkingDirectory=/path/to/transcode-worker
ExecStart=/path/to/transcode-worker/bin/worker
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

Enable and start:
```bash
sudo systemctl enable transcode-worker
sudo systemctl start transcode-worker
sudo systemctl status transcode-worker
```

## Job Specification

The orchestrator sends jobs in this format:

```json
{
  "job_id": "job-123",
  "movie_id": "movie-456",
  "input_source": "/mnt/nas/raw/movie.mp4",
  "output_base": "/mnt/nas/transcoded/movie-456",
  "outputs": [
    {
      "resolution": "1080p",
      "bitrate": "5000k",
      "codec": "h264_nvenc",
      "dest_path": "/mnt/nas/transcoded/movie-456/1080p"
    },
    {
      "resolution": "720p",
      "bitrate": "3000k",
      "codec": "h264_nvenc",
      "dest_path": "/mnt/nas/transcoded/movie-456/720p"
    }
  ],
  "profile": {
    "preset": "fast",
    "hls_segment_duration": 6,
    "audio_codec": "aac",
    "audio_bitrate": "128k"
  }
}
```

## API Endpoints (Orchestrator)

The worker communicates with these orchestrator endpoints:

- `POST /api/v1/workers/register` - Register worker
- `POST /api/v1/workers/heartbeat` - Send health status
- `POST /api/v1/jobs/request` - Request a new job
- `PATCH /api/v1/jobs/{id}` - Update job progress
- `POST /api/v1/jobs/{id}/finalize` - Report job completion

## Troubleshooting

### Worker fails to register
- Check orchestrator URL is correct and accessible
- Verify network connectivity
- Check orchestrator logs

### FFmpeg not found
- Install FFmpeg: `sudo apt install ffmpeg` (Ubuntu/Debian)
- Or build from source with desired encoders

### No GPU acceleration
- Verify GPU drivers are installed
- Check FFmpeg was built with hardware encoder support
- Run: `ffmpeg -encoders | grep nvenc` (for NVIDIA)

### Jobs fail mid-transcoding
- Check NAS mount is accessible
- Verify sufficient disk space in temp directory
- Check FFmpeg logs for encoding errors

### High CPU usage even when idle
- Adjust `heartbeat_interval` to reduce frequency
- Check for background processes consuming resources

## Development

### Project Structure

```
transcode-worker/
├── cmd/
│   └── worker/          # Main application entry point
│       └── main.go
├── internal/
│   ├── client/          # HTTP client for orchestrator communication
│   ├── config/          # Configuration management
│   ├── monitor/         # System monitoring and capability detection
│   ├── server/          # Optional HTTP server (for future features)
│   └── transcoder/      # FFmpeg transcoding logic
├── pkg/
│   └── models/          # Shared data models
├── config-example.yml   # Example configuration
└── go.mod
```

### Running Tests

```bash
make test
```

### Building for Production

```bash
make build-prod
```

This creates an optimized binary with debug symbols stripped.

## What's Implemented

This is a **fully functional first version** with:

1. ✅ **Worker Registration** - Announces itself to orchestrator on startup
2. ✅ **Heartbeat System** - Periodic health reports with CPU/RAM stats
3. ✅ **Capability Detection** - Auto-detects hardware encoders (NVENC, QSV, etc.)
4. ✅ **Job Polling** - Continuously requests jobs when idle
5. ✅ **FFmpeg Transcoding** - Executes multi-rendition HLS encoding
6. ✅ **Progress Tracking** - Real-time updates with percentage and FPS
7. ✅ **Job Completion** - Reports success/failure to orchestrator
8. ✅ **Graceful Shutdown** - Handles SIGINT/SIGTERM and cancels jobs
9. ✅ **NAS Integration** - Reads/writes from shared storage
10. ✅ **Configuration Management** - YAML + environment variables

## License

[Your License Here]

## Contributing

Contributions are welcome! Please open an issue or submit a pull request.
