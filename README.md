# transcode-worker

This project provides a worker service managing heavy transcoding jobs. It is meant for allowing low-powear weak media servers to keep providing high quality data by delegating the heavy video transcoding job to available powerful devices accross a worker registry. 

Devices that act as workers will continuously announce their availability through heartbeats, where they will also display their:
- Hardware capabilites
- Operating system
- Status (`IDLE`, `WORKING`).
  
By being idle, the worker can be assigned transcoding jobs, where essential data as original filepath, resolution and bitrate is given. When working on the task, the worker constantly updates the orchestrator about its progress and also serves a URL for a local stream. 

The idea is that the orchestrator keeps rid of most of the overhead and its only responsible for handling metadata and job routing. 

# Dataflow 
The system operates on a Pull-Push hybrid model to keep the Orchestrator's CPU at a minimum.

1. **Register:** The Worker starts and "shouts" its hardware specs to the Orchestrator.
2. **Heartbeat:** Every 15s, the Worker updates the Orchestrator with its CPU load.
3. **Job Assign:** The Orchestrator sends a JSON task to the Worker's HTTP port.
4. **Process:** The Worker spawns a Goroutine (lightweight thread) to run FFmpeg.
5. **Serve:** The Worker's internal web server hosts the video for the family.
