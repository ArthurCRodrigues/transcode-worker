# System Configuration Overview

The worker uses a hierarchical configuration model. It first looks for a `config.yml` file, but allows any value to be overridden by Environment Variables. This is essential for "low-level" servers, where you might want to change a setting via a startup script without editing files.
The Configuration Precedence:
- **Environment Variables** (Highest priority: CINE_WORKER_ID)
- **Config File** (configs/config.yml)
- **Code Defaults** (Fallback: e.g., heartbeat_seconds: 15)

## Setting Up the `config.yml`
When a user (or you) sets up a new worker node, these are the mandatory fields. Each server a specific purpose within the distributed lifecycle.
```yml
# --- Identity & Networking ---
worker_id: "fedora-desktop-01"        # Unique name in the pool
orchestrator_url: "http://192.168.1.100:8080" # Udoo board IP
port: 8081                           # Port where this worker listens for jobs

# --- Resource Management ---
heartbeat_seconds: 15                # Frequency of health updates
max_concurrent_jobs: 1               # Limit for low-level servers (Udoo = 1, PC = 2+)
log_level: "info"                    # debug, info, warn, error

# --- Storage & Hardware ---
nas_mount_path: "/mnt/nas/movies"    # Local path to the media library
enable_hw_accel: true                # Set to false for CPU-only nodes
temp_dir: "/tmp/cinephilia"          # Where .ts chunks are stored during transcode
```
### Why these settings?
- `worker_id`: The Orchestrator may use this as a PK.
- `nas_mount_path`: The orchestrator will send a relative path (e.g.,`/scifi/Dune.mkv`), and the worker prepends this mount path to find the file locally.
- `max_concurrent_jobs`: Prevents resource exhaustion. On low level workers, setting this to `1` sneures the system doesn`t run out of memory by trying to start two FFmpeg instances.

## Cross-Platform Management
One of the main goals of this project is to allow a worker to be installed in linux and windows machines. Go handles this through **Build Contraints** and the **Standard Library**

**A. The "Path" Problem**
Linux uses `/`, Windows uses `\`. Go solves this with the `path/filepath` package. Instead of hardcoding strings, the system uses:
```go
// Works perfectly on Fedora and Windows
fullPath := filepath.Join(cfg.NASMountPath, job.FileName)
```
**B. Conditional Compilation**
The system manages platform differences by probing the environment at startup:
1. **Binary Search (Not the Algorithm!):** Uses `exec.LookPath("ffmpeg")` to find the executable regardless of whether it's in `/usr/bin` or `C:\ffmpeg\bin`.
2. **Capability Check:** Executes `ffmpeg -encoders` and parses the text
  - If it sees `h264_vaapi`, it flags the node as **Linux-Accelerated**
  - If it sees `h264_videotoolbox`, it flags the node as **Mac-Accelerated**
