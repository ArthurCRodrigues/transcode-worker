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
