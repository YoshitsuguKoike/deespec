# Amazon Linux Deployment Guide

This guide explains how to deploy DeeSpec as a continuously running background service on Amazon Linux 2 or Amazon Linux 2023.

## Table of Contents

- [Overview](#overview)
- [Quick Start](#quick-start)
- [Deployment Methods](#deployment-methods)
  - [Method 1: Automated Setup Script](#method-1-automated-setup-script-recommended)
  - [Method 2: EC2 User Data](#method-2-ec2-user-data)
  - [Method 3: Manual Installation](#method-3-manual-installation)
- [Service Configuration](#service-configuration)
- [Operation](#operation)
- [Monitoring](#monitoring)
- [Troubleshooting](#troubleshooting)
- [Resource Management](#resource-management)

## Overview

This deployment configures DeeSpec to run as a systemd service with:

- **Parallel Execution**: Up to 3 concurrent SBI tasks
- **Auto-Start**: Starts automatically on system boot
- **Auto-Restart**: Recovers automatically from crashes
- **Resource Limits**: Controlled memory and CPU usage
- **Log Management**: Integrated with systemd journald

### Architecture

```
┌─────────────────────────────────────┐
│   Amazon Linux Instance             │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  Web VSCode                  │  │
│  │  (Port 8080)                 │  │
│  └──────────────────────────────┘  │
│                                     │
│  ┌──────────────────────────────┐  │
│  │  DeeSpec Service (systemd)   │  │
│  │  • Parallel: 3 tasks         │  │
│  │  • Interval: 1s              │  │
│  │  • Auto-restart: enabled     │  │
│  └──────────────────────────────┘  │
│                                     │
│  /home/ec2-user/workspace/deespec  │
│  └── .deespec/                     │
│      ├── specs/                    │
│      ├── var/                      │
│      └── setting.json              │
└─────────────────────────────────────┘
```

## Quick Start

### One-Line Installation

```bash
curl -fsSL https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/setup-amazon-linux.sh | bash
```

This command will:
1. Install DeeSpec binary
2. Create working directory
3. Initialize DeeSpec project
4. Configure systemd service
5. Start the service

## Deployment Methods

### Method 1: Automated Setup Script (Recommended)

For existing Amazon Linux instances:

```bash
# Download and run setup script
curl -fsSL https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/setup-amazon-linux.sh | bash
```

Or download and inspect before running:

```bash
wget https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/setup-amazon-linux.sh
less setup-amazon-linux.sh  # Review the script
chmod +x setup-amazon-linux.sh
./setup-amazon-linux.sh
```

### Method 2: EC2 User Data

For automatic setup during EC2 instance launch:

1. **Copy User Data Script**:
   ```bash
   cat scripts/ec2-userdata.sh
   ```

2. **Launch EC2 Instance with User Data**:
   - In AWS Console: Advanced Details → User Data → paste script
   - With AWS CLI:
     ```bash
     aws ec2 run-instances \
       --image-id ami-xxxxx \
       --instance-type t3.medium \
       --key-name your-key \
       --user-data file://scripts/ec2-userdata.sh
     ```

3. **Verify Installation** (after instance starts):
   ```bash
   # SSH into instance
   ssh ec2-user@your-instance-ip

   # Check service status
   sudo systemctl status deespec

   # Check logs
   sudo journalctl -u deespec -n 50
   ```

### Method 3: Manual Installation

For full control over the installation:

#### Step 1: Install DeeSpec

```bash
curl -fsSL https://raw.githubusercontent.com/YoshitsuguKoike/deespec/main/scripts/install.sh | bash
export PATH="$HOME/.local/bin:$PATH"
```

#### Step 2: Create Working Directory

```bash
mkdir -p /home/ec2-user/workspace/deespec
cd /home/ec2-user/workspace/deespec
deespec init
```

#### Step 3: Create systemd Service File

```bash
sudo tee /etc/systemd/system/deespec.service > /dev/null <<'EOF'
[Unit]
Description=DeeSpec Continuous Task Runner (Parallel Mode)
Documentation=https://github.com/YoshitsuguKoike/deespec
After=network-online.target
Wants=network-online.target

[Service]
Type=simple
User=ec2-user
Group=ec2-user
WorkingDirectory=/home/ec2-user/workspace/deespec
ExecStart=/home/ec2-user/.local/bin/deespec run --interval 1s --parallel 3 --auto-fb --log-level info

KillMode=mixed
KillSignal=SIGTERM
TimeoutStopSec=30s

Restart=always
RestartSec=10s
StartLimitInterval=300s
StartLimitBurst=5

MemoryMax=2G
MemoryHigh=1.5G
CPUQuota=60%

StandardOutput=journal
StandardError=journal
SyslogIdentifier=deespec

NoNewPrivileges=true
PrivateTmp=true
ReadWritePaths=/home/ec2-user/workspace/deespec/.deespec

[Install]
WantedBy=multi-user.target
EOF
```

#### Step 4: Enable and Start Service

```bash
sudo systemctl daemon-reload
sudo systemctl enable deespec.service
sudo systemctl start deespec.service
```

#### Step 5: Verify

```bash
sudo systemctl status deespec.service
```

## Service Configuration

### Service Parameters

The service is configured with these parameters:

| Parameter | Value | Description |
|-----------|-------|-------------|
| `--interval` | 1s | Check for new tasks every second |
| `--parallel` | 3 | Run up to 3 SBIs concurrently |
| `--auto-fb` | enabled | Automatically register feedback SBIs |
| `--log-level` | info | Log level (debug/info/warn/error) |

### Customizing Configuration

Edit the service file:

```bash
sudo systemctl edit --full deespec.service
```

Common customizations:

**Increase parallelism** (for larger instances):
```ini
ExecStart=/home/ec2-user/.local/bin/deespec run --interval 1s --parallel 5 --auto-fb
```

**Adjust resource limits** (for t3.large or larger):
```ini
MemoryMax=4G
MemoryHigh=3G
CPUQuota=80%
```

**Change check interval**:
```ini
ExecStart=/home/ec2-user/.local/bin/deespec run --interval 5s --parallel 3 --auto-fb
```

After editing, reload and restart:
```bash
sudo systemctl daemon-reload
sudo systemctl restart deespec.service
```

## Operation

### Service Management

```bash
# Check status
sudo systemctl status deespec

# Start service
sudo systemctl start deespec

# Stop service
sudo systemctl stop deespec

# Restart service
sudo systemctl restart deespec

# Enable auto-start on boot
sudo systemctl enable deespec

# Disable auto-start
sudo systemctl disable deespec
```

### Task Management

```bash
# Navigate to working directory
cd /home/ec2-user/workspace/deespec

# Register new SBI
deespec sbi register --title "Your Task Title" --body "Task description"

# List SBIs
deespec sbi list

# Show SBI details
deespec sbi show <sbi-id>

# Check execution history
deespec sbi history <sbi-id>
```

## Monitoring

### Real-time Logs

```bash
# Follow logs in real-time
sudo journalctl -u deespec -f

# Show last 100 lines
sudo journalctl -u deespec -n 100

# Show logs since today
sudo journalctl -u deespec --since today

# Show only errors
sudo journalctl -u deespec -p err

# Export logs to file
sudo journalctl -u deespec --since "2 hours ago" > deespec-logs.txt
```

### Resource Monitoring

```bash
# Check resource usage
sudo systemctl show deespec -p MemoryCurrent,CPUUsageNSec

# Detailed status
sudo systemctl status deespec -l

# Watch process in real-time
watch -n 1 'sudo systemctl show deespec -p MemoryCurrent,CPUUsageNSec'
```

### Health Checks

```bash
cd /home/ec2-user/workspace/deespec

# Check system health
deespec health check

# View health status
cat .deespec/var/health.json
```

### CloudWatch Integration (Optional)

Install CloudWatch agent to send logs:

```bash
# Install CloudWatch agent
sudo yum install amazon-cloudwatch-agent -y

# Configure to send journald logs
sudo tee /opt/aws/amazon-cloudwatch-agent/etc/amazon-cloudwatch-agent.json <<'EOF'
{
  "logs": {
    "logs_collected": {
      "files": {
        "collect_list": [
          {
            "file_path": "/var/log/messages",
            "log_group_name": "/aws/ec2/deespec",
            "log_stream_name": "{instance_id}/messages"
          }
        ]
      }
    }
  }
}
EOF

sudo systemctl restart amazon-cloudwatch-agent
```

## Troubleshooting

### Service Won't Start

**Check logs**:
```bash
sudo journalctl -u deespec -n 50 --no-pager
```

**Common issues**:

1. **Binary not found**:
   ```bash
   # Verify binary location
   which deespec

   # Update service file if needed
   sudo systemctl edit --full deespec.service
   ```

2. **Working directory doesn't exist**:
   ```bash
   mkdir -p /home/ec2-user/workspace/deespec
   cd /home/ec2-user/workspace/deespec
   deespec init
   ```

3. **Permission issues**:
   ```bash
   # Fix ownership
   sudo chown -R ec2-user:ec2-user /home/ec2-user/workspace/deespec

   # Restart service
   sudo systemctl restart deespec
   ```

### Service Keeps Crashing

**Check crash logs**:
```bash
sudo journalctl -u deespec --since "10 minutes ago"
```

**Check if hitting resource limits**:
```bash
sudo systemctl show deespec -p MemoryCurrent,MemoryMax
```

**Adjust resource limits**:
```bash
sudo systemctl edit --full deespec.service
# Increase MemoryMax if needed
sudo systemctl daemon-reload
sudo systemctl restart deespec
```

### High CPU/Memory Usage

**Monitor resource usage**:
```bash
# Real-time monitoring
top -p $(pgrep -f deespec)

# Or use systemd
sudo systemctl status deespec
```

**Reduce parallelism**:
```bash
sudo systemctl edit --full deespec.service
# Change --parallel 3 to --parallel 1
sudo systemctl daemon-reload
sudo systemctl restart deespec
```

### Logs Not Appearing

**Check journald**:
```bash
# Verify journald is running
sudo systemctl status systemd-journald

# Check journal disk usage
sudo journalctl --disk-usage

# Rotate logs if needed
sudo journalctl --vacuum-time=7d
```

## Resource Management

### Recommended Instance Sizes

| Instance Type | Parallel Tasks | Memory Limit | CPU Quota |
|---------------|----------------|--------------|-----------|
| t3.micro      | 1              | 512M         | 30%       |
| t3.small      | 1-2            | 1G           | 40%       |
| t3.medium     | 2-3            | 2G           | 60%       |
| t3.large      | 3-5            | 4G           | 80%       |
| t3.xlarge     | 5-8            | 8G           | 100%      |

### Coexistence with Other Services

If running alongside Web VSCode or other services:

1. **Adjust resource limits**:
   ```bash
   sudo systemctl edit --full deespec.service
   ```

2. **For t3.medium with VSCode**:
   ```ini
   MemoryMax=1.5G      # Leave 2.5G for VSCode
   CPUQuota=40%        # Leave 60% for VSCode
   ```

3. **For t3.large with VSCode**:
   ```ini
   MemoryMax=4G        # Leave 4G for VSCode
   CPUQuota=60%        # Leave 40% for VSCode
   ```

### Monitoring Resource Impact

```bash
# Check overall system resources
free -h
uptime

# Check DeeSpec specific usage
sudo systemctl show deespec -p MemoryCurrent
ps aux | grep deespec

# Check I/O impact
sudo iotop -p $(pgrep -f deespec)
```

## Uninstallation

To completely remove DeeSpec service:

```bash
# Stop and disable service
sudo systemctl stop deespec.service
sudo systemctl disable deespec.service

# Remove service file
sudo rm /etc/systemd/system/deespec.service
sudo systemctl daemon-reload

# Remove binary (optional)
rm ~/.local/bin/deespec

# Remove working directory (optional - contains your data!)
rm -rf /home/ec2-user/workspace/deespec
```

## See Also

- [DeeSpec Documentation](https://github.com/YoshitsuguKoike/deespec)
- [systemd Documentation](https://www.freedesktop.org/software/systemd/man/systemd.service.html)
- [Amazon Linux User Guide](https://docs.aws.amazon.com/linux/)
