> See [components.md](components.md) before reading this documentation.
```mermaid
graph TD
    %% External Entities
    Orch[External Orchestrator]
    User[Client Device: TV/Phone]
    NAS[(Network Storage / NAS)]

    subgraph "Transcode-Worker (Go Runtime)"
        subgraph "Communication Layer (internal/server)"
            API[HTTP Job Receiver]
            FileSrv[HLS Stream Server]
        end

        subgraph "Orchestration Layer (internal/scheduler)"
            Queue{Job Channel / Queue}
            Monitor[Process Monitor]
        end

        subgraph "Execution Layer (internal/transcoder)"
            CmdGen[Command Factory]
            Exec[OS Executor]
            Prober[Hardware Prober]
        end

        subgraph "Telemetry (internal/heartbeat)"
            Pulse[Heartbeat Loop]
        end

        Config[internal/config]
    end

    %% OS Layer
    FF[(FFmpeg Process)]

    %% Connections: Control Flow
    Orch -- "POST /jobs" --> API
    API -- "Push Job" --> Queue
    Queue -- "Pop Job" --> CmdGen
    CmdGen -- "Prepare exec.Cmd" --> Exec
    Exec -- "Spawn" --> FF

    %% Connections: Data Flow
    NAS -- "Read Source" --> FF
    FF -- "Write .ts Segments" --> FileSrv
    FileSrv -- "Serve HLS" --> User

    %% Connections: Telemetry & Monitoring
    FF -- "stderr (progress)" --> Monitor
    Monitor -- "Status Updates" --> Pulse
    Pulse -- "Status/Telemetry" --> Orch
    
    %% Setup flow
    Config -.-> API
    Config -.-> Pulse
    Prober -.-> Config
```

## Deep Dive: Component-to-Component Interaction
### A. Server &rarr; Scheduler (The Channel)
Instead of the Server calling a function in the Scheduler directly (which would be "Synchronous" and block the network request), it sends the data into a Go Channel.
- **Example:** `chan models.TranscodeJob`
- Creates an assynchronous brdige. The server can return `202 Accepted` to the Orchestrator immediately, while the scheduler processes the job whenever it has CPU space.
### B. Scheduler &rarr; Transcoder (The Wrapper)
The Scheduler holds a reference to the `Transcoder.Engine`
- **Example:** Pointer Receiver calls `engine.BuildCommand(job).
- The engine is stateless, it just knows how to talk to FFmpeg. The Scheduler is stateful, it know if a job is currently running.
### C. Transcoder &rarr; FFmpeg 
Where go leaves its runtime and talks to the OS.
- **Example:** `os/exec` pipes (`StoudtPipe` and `StderrPipe`)
- Logs are "piped" from FFmpeg back into go. This allows the **Monitor** to parse the text and see if FFmpeg is failing or how many frames it has processed.
### D. Heartbeat &rarr; Everywhere (Shared State)
Hearbeat needs to know what everyone's doing to report it to the Orchestrator
- **Example:**: Shared Atomic Variables or Mutexes.
- Since the hearbeat runs in its own **Goroutine**, its crucial to use thread-safe ways to read "Current CPU Usage" or "Active Job ID" without causing race conditions.
