# transcode-worker

This project provides a worker service for managing video transcoding jobs. It is part of an architecture designed for allowing low-power weak media servers to keep providing high quality data by delegating the transcoding job to available powerful devices across a worker registry.

It turns any commodity hardware (RaspberryPi,Linux or Windows pc) into a worker node of a distributed media server.

## Workflow

The worker operates in a pull-based architecture, autonomously discovering its host hardware capabilities and polling the orchestrator for transcoding jobs.

### 1. Continuous Heartbeat & State Synchronization

Upon startup, the worker immediately begins sending heartbeat signals to the orchestrator. These heartbeats serve dual purposes: they keep the worker's registration alive and automatically re-register the worker if the orchestrator restarts. This idempotent design ensures that even if the orchestrator crashes and reboots, the next heartbeat from any worker immediately repopulates the orchestrator's state without manual intervention.

**Heartbeat Payload (POST `/api/v1/workers/heartbeat`):**
```json
{
  "worker_id": "desktop-gaming-pc",
  "status": "IDLE",
  "hardware_stats": {
    "cpu_percent": 15.3,
    "ram_percent": 42.1,
    "is_busy": false,
    "accelerator": {
      "type": "nvenc",
      "available": true,
      "codecs": ["h264_nvenc", "hevc_nvenc"]
    }
  },
  "current_job_id": ""
}
```

The worker discovers and reports:
- **CPU** Architecture and core count
- **GPU Acceleration** by probing `ffmpeg` encoders (NVENC, QSV, VAAPI)
- Real-time telemetry monitoring (CPU/RAM usage)

This data enables the orchestrator to make intelligent job scheduling decisions (e.g., routing 4K HEVC jobs to GPU-accelerated nodes while reserving CPU-only nodes for lighter 720p tasks).

### 2. Job Polling & Assignment

The worker proactively requests work when it's `IDLE` and when the machine is not under heavy load (e.g., someone else is gaming on the PC):

**Job Request Payload (POST `/api/v1/jobs/request`):**
```json
{
  "worker_id": "desktop-gaming-pc",
  "capabilities": {
    "supported_codecs": ["h264_nvenc", "hevc_nvenc", "libx264"],
    "has_gpu": true,
    "gpu_type": "nvidia"
  }
}
```

**Job Assignment Response:**
```json
{
  "job_id": "job-1767791635",
  "movie_id": "movie-456",
  "input": {
    "source_url": "movies/sample_3840x2160.mkv",
    "format": "mkv"
  },
  "outputs": [
    {
      "resolution": "1080p",
      "bitrate": "5000k",
      "codec": "h264_nvenc",
      "dest_path": "processed/sample/1080p/"
    },
    {
      "resolution": "720p",
      "bitrate": "2500k",
      "codec": "h264_nvenc",
      "dest_path": "processed/sample/720p/"
    }
  ],
  "hls_settings": {
    "master_playlist_name": "index.m3u8",
    "segment_time": 6
  }
}
```

### 3. Progress Reporting

During transcoding, the worker sends periodic progress updates:

**Progress Update Payload (PATCH `/api/v1/jobs/{job_id}`):**
```json
{
  "worker_id": "desktop-gaming-pc",
  "status": "PROCESSING",
  "progress": 45.8,
  "current_fps": 87.3,
  "eta_sec": 120
}
```

### 4. Job Completion

Upon completion (success or failure), the worker finalizes the job:

**Success Payload (POST `/api/v1/jobs/{job_id}/finalize`):**
```json
{
  "status": "COMPLETED",
  "manifest_url": "/processed/sample/720p/index.m3u8",
  "metrics": {
    "total_time_ms": 245680
  }
}
```

**Failure Payload:**
```json
{
  "status": "FAILED",
  "error_msg": "input file does not exist: /mnt/nas/movies/sample.mkv",
  "metrics": {
    "total_time_ms": 1250
  }
}
```

### Video HLS Pipeline

The worker treats transcoding as an atomic transaction. The pipeline follows the steps below:
- **Ingest:** Reads raw media directly from the NAS
- **Process:** Executes FFMpeg to generate HLS playlists and segments.
- **Stage:** Writes all artifacts to a local temporary directory.
- **Commit:** Performs a bulk transfer to the NAS only upon succesful completion.

## Setting up the worker
Before running the worker, make sure to setup the necessary [configurations](config-example.yml). 
### Prerequisites
- **Go 1.21+** for building
- **FFmpeg** with desired encoders (h264_nvenc, libx264, etc.)
- **FFprobe** for media inspection
- **Mounted NAS storage** for accessing raw files and storing outputs

Setting up the worker as background service is differet depending on the OS you're in. The documentation provides guides for linux and windows machines:
- [linux](docs/setup/linux.md)
- [windows](docs/setup/windows.md)





