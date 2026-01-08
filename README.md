# transcode-worker

This project provides a worker service for managing video transcoding jobs. It is part of an architecture designed for allowing low-power weak media servers to keep providing high quality data by delegating the transcoding job to available powerful devices across a worker registry.

It turns any commodity hardware (RaspberryPi,Linux or Windows pc) into a worker node of a distributed media server. 

It does video segmentation of the input source and transcode heavy formats into web friendly ones.

## Workflow

The worker operates in a pull-based architecture, autonomously discovering its host hardware capabilities and synchronizing with the orchestrator for transcoding jobs.

### 1. Initial Registration

On startup, the worker performs a deep inspection of its host environment and registers once with the orchestrator, declaring its static capabilities:

**Registration Payload (POST `/api/v1/workers/register`):**
```json
{
  "worker_id": "desktop-gaming-pc",
  "capabilities": {
    "supported_codecs": ["h264_nvenc", "hevc_nvenc", "libx264"],
    "has_gpu": true,
    "gpu_type": "nvidia",
    "max_resolution": "4k"
  }
}
```

The worker discovers:
- **CPU** Architecture and core count  
- **GPU Acceleration** by probing `ffmpeg` encoders (NVENC, QSV, VAAPI)
- **Supported Codecs** for hardware and software encoding

### 2. Continuous Sync Loop (Heartbeat + Job Assignment)

The worker continuously syncs with the orchestrator through a unified bidirectional endpoint. This replaces the old pattern of separate heartbeat and job polling loops:

**Sync Payload (POST `/api/v1/workers/sync`):**
```json
{
  "worker_id": "desktop-gaming-pc",
  "status": "IDLE",
  "hardware_stats": {
    "cpu_percent": 15.3,
    "ram_percent": 42.1,
    "is_busy": false
  },
  "current_job_id": ""
}
```

**Sync Response:**
```json
{
  "ack": true,
  "assigned_job": {
    "job_id": "job-1767791635",
    "movie_id": "movie-456",
    "input": {
      "source_url": "movies/sample_3840x2160.mkv",
      "format": "mkv"
    },
    "outputs": [
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
}
```

The sync loop serves dual purposes:
- **When BUSY**: Acts as a heartbeat to keep the worker registered
- **When IDLE**: Receives job assignments directly in the response

**Lazy Re-Registration**: If the orchestrator restarts and loses state, the next sync will fail with HTTP 404. The worker automatically re-registers and retries the sync, ensuring zero-downtime recovery.

**Serial Execution Model**: The worker processes exactly one job at a time. When a job is assigned, the worker status changes to `BUSY` and the `current_job_id` field is populated. During this time, the sync loop continues but no new jobs are assigned. This ensures:
- Predictable resource usage (no job contention)
- Simpler error handling and recovery
- Clear reporting of what the worker is doing at any moment

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





