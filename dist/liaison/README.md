# Liaison Systemd Service

This directory contains systemd service files and installation scripts for the Liaison edge computing management service.

## Files

- `liaison.service` - Systemd service unit file
- `install.sh` - Installation script
- `uninstall.sh` - Uninstallation script
- `README.md` - This documentation

## Installation

### Prerequisites

- Linux system with systemd
- Root privileges
- Liaison binary compiled and ready
- WebDesktop support uses `guacd`. `make package` prepares bundled `bin/guacd` plus its runtime from `guacamole/guacd:1.5.5`; `install.sh` uses the bundled runtime first, then falls back to an existing system `guacd`, package-manager install, or Docker sidecar if needed.

### Quick Installation

1. Make the installation script executable:
   ```bash
   chmod +x install.sh
   ```

2. Run the installation script as root:
   ```bash
   sudo ./install.sh
   ```

3. Copy the liaison binary to the installation directory:
   ```bash
   sudo cp /path/to/liaison /opt/liaison/bin/
   sudo chown liaison:liaison /opt/liaison/bin/liaison
   sudo chmod +x /opt/liaison/bin/liaison
   ```

4. Copy the configuration file:
   ```bash
   sudo cp /path/to/liaison.yaml /opt/liaison/conf/
   sudo chown liaison:liaison /opt/liaison/conf/liaison.yaml
   ```

5. Start the service:
   ```bash
   sudo systemctl start liaison
   ```

### Manual Installation

If you prefer to install manually:

1. Create the service user:
   ```bash
   sudo useradd --system --no-create-home --shell /bin/false liaison
   ```

2. Create directories:
   ```bash
   sudo mkdir -p /opt/liaison/{bin,conf,data,logs}
   sudo chown -R liaison:liaison /opt/liaison
   sudo chmod 750 /opt/liaison/{data,logs}
   ```

3. Copy the service file:
   ```bash
   sudo cp liaison.service /etc/systemd/system/
   sudo systemctl daemon-reload
   sudo systemctl enable liaison
   ```

## TLS/HTTPS Configuration

### Binding to Privileged Ports (443, 80, etc.)

The service file is configured with `CAP_NET_BIND_SERVICE` capability, which allows the service to bind to ports below 1024 (like 443 for HTTPS) without running as root.

**Important Notes:**
- The service uses `AmbientCapabilities=CAP_NET_BIND_SERVICE` to allow binding to privileged ports
- `NoNewPrivileges=false` is required when using AmbientCapabilities
- This is more secure than running the entire service as root

### Alternative: Using Non-Privileged Ports

If you prefer not to use capabilities, you can:
1. Use a non-privileged port (e.g., 8443 instead of 443)
2. Use iptables to forward traffic:
   ```bash
   sudo iptables -t nat -A PREROUTING -p tcp --dport 443 -j REDIRECT --to-port 8443
   ```

### Alternative: Using setcap (Not Recommended)

You can also set the capability directly on the binary:
```bash
sudo setcap 'cap_net_bind_service=+ep' /opt/liaison/bin/liaison
```
However, this is less flexible than using systemd capabilities.

## Service Management

### Basic Commands

```bash
# Start the service
sudo systemctl start liaison-guacd frontier liaison

# Stop the service
sudo systemctl stop liaison frontier liaison-guacd

# Restart the service
sudo systemctl restart liaison-guacd frontier liaison

# Check service status
sudo systemctl status liaison frontier liaison-guacd

# View service logs
sudo journalctl -u liaison -f
sudo journalctl -u liaison-guacd -f

# Enable service to start on boot
sudo systemctl enable liaison

# Disable service from starting on boot
sudo systemctl disable liaison
```

### Log Management

The service logs to systemd journal by default. You can view logs using:

```bash
# View recent logs
sudo journalctl -u liaison

# Follow logs in real-time
sudo journalctl -u liaison -f

# View logs from today
sudo journalctl -u liaison --since today

# View logs with timestamps
sudo journalctl -u liaison -o short-iso
```

## Configuration

The service expects the configuration file at `/opt/liaison/conf/liaison.yaml`. Make sure to:

1. Update the configuration file according to your environment
2. Ensure the database path is writable by the liaison user
3. Configure the correct frontier server addresses
4. Set appropriate log levels and file paths

### Example Configuration

```yaml
manager:
  listen:
    addr: 0.0.0.0:8080
    network: tcp
    tls:
      enable: false
  db: /opt/liaison/data/liaison.db
  guacd_addr: 127.0.0.1:4822
  guacd_bridge_addr: 127.0.0.1:0
  guacd_bridge_host: 127.0.0.1
frontier:
  controlplane_url: http://127.0.0.1:30010
  dial:
    addrs:
      - 127.0.0.1:30011
    network: tcp
    tls:
      enable: false
log:
  level: info
  file: /opt/liaison/logs/liaison.log
  maxsize: 100
  maxrolls: 10
```

`guacd` is intentionally bound to loopback in the generated service so it is not exposed publicly. The installer prefers the bundled `/opt/liaison/bin/guacd` wrapper and `/opt/liaison/guacd-rootfs` runtime. If they are missing, the installer tries an existing system `guacd`, package-manager install, then a Docker sidecar using host networking and the same loopback bind. Liaison still starts if guacd cannot be installed; only WebDesktop is unavailable until guacd is installed.

## Security Features

The service is configured with several security features:

- Runs as a dedicated system user (`liaison`)
- Uses `NoNewPrivileges` to prevent privilege escalation
- Has restricted file system access with `ProtectSystem=strict`
- Uses `PrivateTmp` for isolated temporary directories
- Has resource limits to prevent resource exhaustion
- Restricts various system capabilities

## Troubleshooting

### Service Won't Start

1. Check the service status:
   ```bash
   sudo systemctl status liaison
   ```

2. Check the logs:
   ```bash
   sudo journalctl -u liaison --no-pager
   ```

3. Verify the binary exists and is executable:
   ```bash
   ls -la /opt/liaison/bin/liaison
   ```

4. Check file permissions:
   ```bash
   ls -la /opt/liaison/
   ```

### Permission Issues

1. Ensure the liaison user owns the installation directory:
   ```bash
   sudo chown -R liaison:liaison /opt/liaison
   ```

2. Check that the configuration file is readable:
   ```bash
   sudo -u liaison cat /opt/liaison/conf/liaison.yaml
   ```

### Database Issues

1. Ensure the database directory is writable:
   ```bash
   sudo chmod 750 /opt/liaison/data
   sudo chown liaison:liaison /opt/liaison/data
   ```

2. Check disk space:
   ```bash
   df -h /opt/liaison/data
   ```

## Uninstallation

To remove the service:

1. Make the uninstallation script executable:
   ```bash
   chmod +x uninstall.sh
   ```

2. Run the uninstallation script as root:
   ```bash
   sudo ./uninstall.sh
   ```

The script will:
- Stop and disable the service
- Remove the systemd service file
- Optionally remove the installation directory
- Optionally remove the service user

## Directory Structure

After installation, the directory structure will be:

```
/opt/liaison/
├── bin/
│   └── liaison          # Liaison binary
├── conf/
│   └── liaison.yaml     # Configuration file
├── data/
│   └── liaison.db       # Database file
└── logs/
    └── liaison.log      # Log file
```

## Support

For issues and questions:
- Check the service logs: `sudo journalctl -u liaison -f`
- Verify configuration: `sudo -u liaison /opt/liaison/bin/liaison -h`
- Review system resources: `systemctl status liaison`

## AI output language

On a fresh installation, the installer recommends Simplified Chinese (`zh`)
for mainland China timezones and English (`en`) for other or unknown timezones.
An interactive installation lets you confirm the choice. For unattended setup:

```bash
sudo env AGENT_OUTPUT_LANGUAGE=en bash install.sh
```

This is a fixed product-wide default, not a browser preference. Existing
installations keep their previous default (legacy installations: Chinese).
Saved model settings take precedence; administrators can change the language
in Settings → Models. Upgrading does not override those saved settings.
