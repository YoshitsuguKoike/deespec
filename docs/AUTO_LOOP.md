# Auto Loop

## systemd on Linux

### Service Unit File (deespec-run.service)

```ini
[Unit]
Description=Run deespec once

[Service]
Type=oneshot
WorkingDirectory=/path/to/project
ExecStart=/usr/local/bin/deespec run --once
```

### Timer Unit File (deespec-run.timer)

```ini
[Unit]
Description=Run deespec every 5 minutes

[Timer]
OnBootSec=1min
OnUnitActiveSec=5min
Unit=deespec-run.service

[Install]
WantedBy=timers.target
```

### Installation

```bash
# Copy unit files
sudo cp deespec-run.service /etc/systemd/system/
sudo cp deespec-run.timer /etc/systemd/system/

# Reload systemd
sudo systemctl daemon-reload

# Enable and start timer
sudo systemctl enable deespec-run.timer
sudo systemctl start deespec-run.timer

# Check status
sudo systemctl status deespec-run.timer
sudo journalctl -u deespec-run.service -f
```

## macOS with launchd

### Launch Agent Plist (com.deespec.run.plist)

```xml
<?xml version="1.0" encoding="UTF-8"?>
<!DOCTYPE plist PUBLIC "-//Apple//DTD PLIST 1.0//EN" "http://www.apple.com/DTDs/PropertyList-1.0.dtd">
<plist version="1.0">
<dict>
    <key>Label</key>
    <string>com.deespec.run</string>
    <key>ProgramArguments</key>
    <array>
        <string>/usr/local/bin/deespec</string>
        <string>run</string>
        <string>--once</string>
    </array>
    <key>WorkingDirectory</key>
    <string>/path/to/project</string>
    <key>StartInterval</key>
    <integer>300</integer>
    <key>StandardOutPath</key>
    <string>/tmp/deespec.out</string>
    <key>StandardErrorPath</key>
    <string>/tmp/deespec.err</string>
</dict>
</plist>
```

### Installation

```bash
# Copy plist file
cp com.deespec.run.plist ~/Library/LaunchAgents/

# Load the agent
launchctl load ~/Library/LaunchAgents/com.deespec.run.plist

# Check status
launchctl list | grep deespec
```

## Simple cron job

```bash
# Edit crontab
crontab -e

# Add this line to run every 5 minutes
*/5 * * * * cd /path/to/project && /usr/local/bin/deespec run --once >> /var/log/deespec.log 2>&1
```