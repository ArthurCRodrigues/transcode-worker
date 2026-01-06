# System Components

## 1. Core Infrastructure (`/cmd` and `/internal/config`)

### Entrypoint (`main.go`)
The entry point is responsible for the **Dependency Injection** and **Lifecycle Management**. It initializes the shared context (for graceful shutdowns) and wires the disparate services (Heartbeat, Server, Transcoder) together. It ensures that if the OS sends an interrupt signal, all background processes are killed before the app exits.

### Config Manager (`config.go`)
Uses a layered approach to define the node's behavior. It prioritizes environment variables over **YAML** files, allowing for dynamic configuration without disk writes.
- **Path Mapping**: Translates local directory structures to the common network storage paths.
- **Capability Flagging**: Stores hardware limits (max CPU cores allowed, hardware acceleration toggles).

## 2. Telemetry % Identity (`internal/heartbeat`)
Responsible for reporting the node's health and availability.

### Heartbeat Service 
A background loop driven by a `time.Ticker`. It doesn't just send an "I'm alive" signal; it sends a Node State payload.
- **State Machine**: Transitions the worker between `IDLE`, `BUSY`, `ERROR`, and `MAINTENANCE` states
- **Telemetry Aggregator**: Before each pulse, it gathers real-time metrics:
  - **CPU Load:** Current percentage used
  - **Memory Pressure:** Available vs used RAM.
  - **Process Count:** Number of active FFmpeg threads
## 3. Communication Layer (`internal/server`)
Exposes a minimal REST API that the Orchestrator uses to push instructions.

### Job Receiver
An HTTP server optimized for low latency
- **Endpoint `/jobs` (POST):** Validades incoming JSON job definitions against the `pkg/models` schema.
- **Admission Control:** Checks the current node state. If the worker is already at `max_concurrent_jobs`, it rejects the assignmnet with a `429 Too Many Requests` or `503 Service Unavailable`, forcing the orchestrator to pick a different node.

### Stream Server
Specialized file server that provided the JIT video segments.
- **HLS Provisioning:** Server `.m3u8` playlists and `.ts` segments.
- **Range Request Support:** Allows clients (like a Smart TV) to seek through a video by requesting specific byte ranges of the transcoded output.

## 4. Execution Engine (`/internal/transcoder`)
Most complext component, handles the interface between Go and the OS kernel. 

### FFmpeg Wrapper
This component manages the **Process Lifecycle**
- **Command Factory:** Dynamically construcs the CLI arguments based on the node's OS and the hardware accelaration available (VAAPI,NVENC, or VideoToolbox)
- **Signal Handling:** Wraps the process in a context. If a job is cancelled by the user on their phone, the worker sends a `SIGKILL` or `SIGTERM` to the specific PID (Process ID) to stop resource consumption immediately.

### Hardware Prober
A diagnostic component that runs once at startup
- **Codec Discovery:** Runs `ffmpeg -enconders` and parses the output to see if high-effiency encoders are supported.
- **Path Verification:** Ensure the worker has write permissions to the output directory and read permissions to the NAS (the place where the media asset is)

## 5. Job Orchestration (`/interna/scheduler`)
Stabilishes a control logic that prevents a low level server from crashing under load.

### Task Queue
A synchronized channel that acts as a buffer. Even if the Orchestrator sends 3 jobs at once, the Scheduler ensures they are processed based on the node's capability (e.g., serial execution for a Udoo Quad, parallel for a Fedora Gaming PC).

### Progress Monitor
Parses the `stderr` output of FFmpeg in real-time. It looks for "frame=" and "time=" strings to calculate the transcoding percentage, which is then sent back to the Orchestrator so the user can see a progress bar.


## Component Interaction Map

| Component     | Responsibility                 | Interacts With             |
|---------------|--------------------------------|----------------------------|
| Config        | Setup & Identity               | All Components             |
| Heartbeat     | Status & Telemetry Reporting   | Orchestrator (Client-side) |
| Job Receiver  | Assignment Ingestion           | Orchestrator (Server-side) |
| Scheduler     | Internal Task Routing          | Transcoder & Receiver      |
| Transcoder    | Heavy Lifting (FFmpeg Wrapper) | OS / FFmpeg / Hardware     |
| Stream Server | Final Media Delivery           | End-User Device (TV/Phone) |
