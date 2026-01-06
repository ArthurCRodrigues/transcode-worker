# transcode-worker

This project provides a worker service managing heavy transcoding jobs. It is meant for allowing low-powear weak media servers to keep providing high quality data by delegating the heavy video transcoding job to available powerful devices accross a worker registry. 

Devices that act as workers will continuously announce their availability through heartbeats, where they will also display their:
- Hardware capabilites
- Operating system
- Status (idle, working).
  
By being idle, the worker can be assigned transcoding jobs, where essential data as original filepath, resolution and bitrate is given. When working on the task, the worker constantly updates the orchestrator about its progress and also serves a URL for a local stream. 

The idea is that the orchestrator keeps rid of most of the overhead and its only responsible for handling metadata and job routing. 

# Communication Examples

## 1. Worker Registration and Heartbeat
**Purpose:** The Worker tells the Orchestrator: "I'm here, I'm on Windows, and I have an NVIDIA GPU."
```json
{
  "worker_id": "gaming-pc-room",
  "ip_address": "192.168.1.50",
  "port": 8080,
  "os": "windows",
  "hw_accel": ["nvenc", "cuda", "d3d11va"],
  "status": "idle",
  "cpu_usage": 5
}
```
## 2. Job Assignmnent (The "Task")
**Purpose:** The Orchestrator tells a specific Worker: "Start transcoding this file at this timestamp."
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "source_path": "movies/sci-fi/interstellar.mkv",
  "resolution": "1080p",
  "bitrate": "10M",
  "start_time": 0,
  "is_mesh_mode": false
}
```
## 3. Job Status Update
**Purpose:** The Worker tells the Orchestrator: "I'm 45% done, and here is the URL for the client to watch."
```json
{
  "job_id": "550e8400-e29b-41d4-a716-446655440000",
  "state": "processing",
  "progress": 12.5,
  "fps": 48,
  "stream_url": "http://192.168.1.50:8080/hls/interstellar/index.m3u8"
}
```

# Dataflow 
The system operates on a Pull-Push hybrid model to keep the Orchestrator's CPU at a minimum.

**1.Register:** The Worker starts and "shouts" its hardware specs to the Udoo.

**Heartbeat:** Every 15s, the Worker updates the Udoo with its CPU load.

**Job Assign:** Udoo sends a JSON task to the Worker's HTTP port.

**Process:** The Worker spawns a Goroutine (lightweight thread) to run FFmpeg.

**Serve:** The Worker's internal web server hosts the video for the family.
