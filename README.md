# transcode-worker

This project provides a worker service managing heavy transcoding jobs. It is part of an architecture designed for allowing low-power weak media servers to keep providing high quality data by delegating the heavy video transcoding job to available powerful devices across a worker registry.

It turns any commodity hardware (RaspberryPi,Linux or Windows pc) into a node in a unified transcoding grid.

## Workflow

The worker operates in a pull based architecture, autonomously discovering its host hardware capabilites and polling the orchestrator for transcoding jobs. 

Upon startup, the worker performs a deep inspection of its host environment. It manages to discover:
- **CPU** Architecture and core count
- **GPU Acceleration** by probing `ffmpeg` encoders
- Real time telemetry monitoring (CPU/RAM usage)

All this data is sent to the orchestrator in heartbeats (which shows that the worker is active) so that it can make intelligent job schedulling decisions (e,g. routing 4K HEVC jovs to GPU accelerated nodes while reserving CPU only nodes for lighter 720p tasks)

For performing jobs, the worker proactively requests work when it's `IDLE` and when the machine is not under heavy load (ex: someone else is gaming in the pc).

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

Setting up the worker as background service is differet depending on the OS you're in. In this README, i will cover the linux setup, but you can check windows setup [here](docs/setup_windows.md)




