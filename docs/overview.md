# System Overview and Lifecycle

The project is designed as a low-overhead, platform-agnostic node. It is engineered to perform resource-intensive video transformations on behalf of an orchestrator, specifically optimized to run on constrained hardware (like the Udoo Quad) while remaining powerful enough to utilize modern gaming hardware (Fedora/Windows).

The worker operates as a "Stateful Execution Node." It does not hold the "source of truth" for the media library; instead, it provides computational power to the orchestrator.

**The Lifecycle:**
1. **Bootstrapping:** The worker loads its config.yml, verifies the local FFmpeg installation, and generates a hardware profile.
2. **Health Signaling (Heartbeat):** The worker begins a background loop, pushing its identity and hardware telemetry to the Orchestrator.
3. **Job Ingestion:** The worker listens on a local port for POST /jobs assignments from the Orchestrator.
4. **Execution:** The worker spawns an isolated FFmpeg process, mapping network storage to local paths.
5. **Provisioning:** The worker hosts the resulting stream segments via a lightweight HTTP server for client consumption.

## Hardware Discovery
To allow the Orchestrator to make intelligent scheduling decisions, the worker must "know itself."

### **Hardware Probing**

The worker utilizes a combination of Go's `runtime` package and OS-level syscalls to gather stats:

- **CPU Topology:** Detects core count and current frequency to determine how many simultaneous "threads" FFmpeg should use.
- **Memory Footprint:** Monitors available RAM. If the free memory drops below a threshold (critical for 1GB devices), the worker flags itself as BUSY.
- **Accelerator Detection:** Linux/Fedora: Probes /dev/dri/renderD128 to see if VAAPI (Intel/AMD) or NVENC (NVIDIA) is available.
  - **macOS:** Checks for VideoToolbox support.
  - **Windows:** Checks for DirectX Video Acceleration (DXVA).

### **Telemetry Propagation**

These stats are bundled into a JSON payload and sent during the Heartbeat. This allows the Orchestrator to say: "Send the 4K HEVC file to the Fedora PC because it has a GPU, but send the 720p file to the Udoo Board because it only has CPU capacity."

## Transcoding Engine
The core functionality is a wrapper around the FFmpeg binary.

Instead of loading the entire movie into memory (which would crash a low-level server), the worker uses a **streaming pipeline**:
1. **Input:** The worker accesses the source file via a NAS mount (SMB/NFS) defined in nas_mount_path.
2. **Processing:** Go's os/exec executes FFmpeg with high-efficiency flags:
  - `-preset veryfast`: To prioritize speed over file size.
  - `-g 60`: Setting specific GOP (Group of Pictures) sizes to ensure HLS segments are cut at clean intervals.
3. **Segmentation:** The worker utilizes the hls muxer to break the video into .ts chunks while it is still being processed.


## Serving the Transcoded Video
Unlike a standard file server, the worker serves video as it is being created. (JIT Delivery)

The worker uses Go's `http.FileServer` or a custom `http.Handler` to serve a specific directory.
- **The `.m3u8` Playlist:** The Orchestrator points the client's device to the worker's URL (e.g., `http://192.168.1.50:8081/stream.m3u8`).
- **Chunk Delivery:** The client (TV) reads the playlist and begins requesting .ts segments. The worker serves these chunks directly from its local temp storage or RAM-disk.
- **Concurrency:** Go’s HTTP server is natively concurrent; it can serve the same video chunks to multiple users simultaneously with minimal overhead.

## Overhead Reduction Strategies
To ensure stability on "low-level" hardware, the project implements:
- **Process Niceness:** On Unix systems, FFmpeg is started with a higher "nice" value, ensuring it doesn't starve the OS of CPU cycles.

- **Zombie Process Prevention:** Go's `cmd.Wait()` and `context` cancellation are used to ensure that if a transcode is cancelled, the FFmpeg process is killed immediately, freeing up RAM.

- **Zero-Copy Routing:** Where possible, the worker serves files directly from the kernel cache using `Sendfile` logic, bypassing user-space memory copies.


## Conclusion

This project transforms a simple PC or single-board computer into a high-utility node. By abstracting the complexity of FFmpeg and hardware differences behind a clean Go interface, we create a scalable pool of workers capable of powering a family-wide streaming ecosystem.
