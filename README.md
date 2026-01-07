# transcode-worker

This project provides a worker service for managing video transcoding jobs. It is part of an architecture designed for allowing low-power weak media servers to keep providing high quality data by delegating the transcoding job to available powerful devices across a worker registry.

It turns any commodity hardware (RaspberryPi,Linux or Windows pc) into a worker node of a distributed media server.

## Workflow

The worker operates in a pull based architecture, autonomously discovering its host hardware capabilites and polling the orchestrator for transcoding jobs. 

Upon startup, the worker performs a deep inspection of its host environment. It manages to discover:
- **CPU** Architecture and core count
- **GPU Acceleration** by probing `ffmpeg` encoders
- Real time telemetry monitoring (CPU/RAM usage)

All this data is sent to the orchestrator in heartbeats (which shows that the worker is active) so that it can make intelligent job schedulling decisions (e.g. routing 4K HEVC jovs to GPU accelerated nodes while reserving CPU only nodes for lighter 720p tasks)

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

### Linux Setup (systemd)

#### 1. Build the Worker

First, build the worker binary:

```bash
cd /path/to/transcode-worker
go build -o bin/worker cmd/worker/main.go
```

Or use the Makefile:

```bash
make build
```

#### 2. Configure the Worker

Copy and edit the configuration file:

```bash
cp config-example.yml config.yml
nano config.yml
```

Update the following settings:

```yaml
orchestrator_url: "http://192.168.1.100:8080"  # Your orchestrator IP
nas_mount_path: "/mnt/nas"                      # Your NAS mount point
temp_dir: "/tmp/transcode-worker"               # Fast local storage
worker_id: ""                                   # Leave empty to use hostname
heartbeat_interval: 10s
max_concurrent_jobs: 1
log_level: "info"
```



#### 3. Create systemd Service

Create a systemd service file:

```bash
sudo nano /etc/systemd/system/transcode-worker.service
```

Paste the following configuration:

```ini
[Unit]
Description=Transcode Worker Service
Documentation=https://github.com/ArthurCRodrigues/transcode-worker
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=your_username
Group=your_username
WorkingDirectory=/path/to/transcode-worker
ExecStart=/path/to/transcode-worker/bin/worker

# Environment variables (optional - can also use config.yml)
Environment="WORKER_ORCHESTRATOR_URL=http://192.168.1.100:8080"
Environment="WORKER_NAS_MOUNT_PATH=/mnt/nas"
Environment="WORKER_TEMP_DIR=/tmp/transcode-worker"

# Restart configuration
Restart=always
RestartSec=10

# Resource limits (optional but recommended)
LimitNOFILE=65536
Nice=10

# Logging
StandardOutput=journal
StandardError=journal
SyslogIdentifier=transcode-worker

[Install]
WantedBy=multi-user.target
```

**Important:** Replace:
- `your_username` with your actual username
- `/path/to/transcode-worker` with the actual path

#### 4. Enable and Start the Service

```bash
# Reload systemd to recognize the new service
sudo systemctl daemon-reload

# Enable the service to start on boot
sudo systemctl enable transcode-worker

# Start the service
sudo systemctl start transcode-worker

# Check the status
sudo systemctl status transcode-worker
```

#### 5. Verify the Worker is Running

Check the logs to ensure the worker registered successfully:

```bash
# View recent logs
sudo journalctl -u transcode-worker -n 50

# Follow logs in real-time
sudo journalctl -u transcode-worker -f

# Filter by time
sudo journalctl -u transcode-worker --since "5 minutes ago"
```

You should see output like:

```
Starting transcode worker: your-hostname
Orchestrator URL: http://192.168.1.100:8080
NAS Mount Path: /mnt/nas
Registering with orchestrator...
Successfully registered as worker: your-hostname
Starting heartbeat loop (interval: 10s)
Starting job polling loop...
```

#### 6. Managing the Service

```bash
# Stop the worker
sudo systemctl stop transcode-worker

# Restart the worker
sudo systemctl restart transcode-worker

# Disable auto-start on boot
sudo systemctl disable transcode-worker

# View service configuration
sudo systemctl cat transcode-worker

# Check if service is enabled
sudo systemctl is-enabled transcode-worker
```

For Windows setup instructions, see [docs/setup_windows.md](docs/setup_windows.md).




