# Linux Setup (systemd)

## 1. Build the Worker

First, build the worker binary:

```bash
cd /path/to/transcode-worker
go build -o bin/worker cmd/worker/main.go
```

Or use the Makefile:

```bash
make build
```

## 2. Configure the Worker

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
sync_interval: 10s                              # How often to sync with orchestrator
log_level: "info"
```



## 3. Create systemd Service

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

## 4. Enable and Start the Service

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

## 5. Verify the Worker is Running

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

## 6. Managing the Service

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

