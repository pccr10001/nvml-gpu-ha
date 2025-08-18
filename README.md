# NVML GPU Home Assistant Monitor

A lightweight Go application that monitors NVIDIA GPU metrics using NVML (NVIDIA Management Library) and publishes them to Home Assistant via MQTT with automatic discovery support.

## Features

- **Real-time GPU monitoring** using NVIDIA's official go-nvml library
- **Home Assistant MQTT Discovery** - sensors automatically appear in HA
- **Multiple GPU support** - monitors all available NVIDIA GPUs
- **Configurable polling interval** - adjust monitoring frequency
- **Request protection** - prevents overlapping NVML requests that can cause slowdowns
- **Concurrent monitoring** - parallel GPU monitoring for better performance
- **MQTT LWT (Last Will and Testament)** support for availability tracking
- **Systemd service** support for automatic startup
- **Docker support** with NVIDIA Container Runtime

## Monitored Metrics

For each GPU, the following sensors are created in Home Assistant:

- **Power Draw** (Watts) - Current power consumption
- **Performance Level** (P0/P8/etc.) - Current P-State
- **VRAM Usage** (%) - Memory utilization percentage  
- **GPU Utilization** (%) - GPU core usage percentage
- **GPU Temperature** (°C) - Current GPU temperature

## GPU Naming Convention

GPUs appear in Home Assistant with the format: `{HOSTNAME} {PCI ID} - NVIDIA {MODEL} {VRAM}`

Example: `MY-SERVER 00:01:00.0 - NVIDIA GeForce RTX 3080 10GB`

## Requirements

### System Requirements
- Linux (Ubuntu, CentOS, etc.) for production use
- NVIDIA GPU with driver version 440.33 or newer
- NVIDIA Management Library (libnvidia-ml.so)
- Uses official [NVIDIA go-nvml](https://github.com/NVIDIA/go-nvml) library

### Software Requirements
- Go 1.21+ (for building from source)
- MQTT broker (Mosquitto, Home Assistant built-in, etc.)
- Home Assistant with MQTT integration

## Installation

### Option 1: Download Pre-built Binary

```bash
# Download the latest release
wget https://github.com/pccr10001/nvml-gpu-ha/releases/latest/download/nvml-gpu-ha-linux-amd64
chmod +x nvml-gpu-ha-linux-amd64
sudo mv nvml-gpu-ha-linux-amd64 /usr/local/bin/nvml-gpu-ha
```

### Option 2: Build from Source

```bash
# Clone the repository
git clone https://github.com/pccr10001/nvml-gpu-ha.git
cd nvml-gpu-ha

# Build and install
make build-linux
sudo cp nvml-gpu-ha-linux-amd64 /usr/local/bin/nvml-gpu-ha
```

### Option 3: Docker

```bash
# Pull the image
docker pull ghcr.io/pccr10001/nvml-gpu-ha:latest

# Or build locally
docker build -t nvml-gpu-ha .
```

## Configuration

### Configuration File (Recommended)

The application reads configuration from `/etc/nvml-gpu-ha.conf` in TOML format. Command line flags can override config file settings.

#### Example Configuration File

```toml
# /etc/nvml-gpu-ha.conf

# Hostname prefix for GPU names (optional, uses system hostname if not specified)
hostname = "MY-SERVER"

# MQTT Broker Configuration
mqtt_host = "192.168.1.100"
mqtt_port = 1883
mqtt_username = "homeassistant"
mqtt_password = "your_secure_password"

# MQTT Options
mqtt_lwt_enable = true
mqtt_retain = true

# Monitoring Settings  
polling_period = 30
```

#### Create Configuration File

```bash
# Copy example config
sudo cp nvml-gpu-ha.conf.example /etc/nvml-gpu-ha.conf

# Edit configuration
sudo nano /etc/nvml-gpu-ha.conf

# Set appropriate permissions
sudo chmod 600 /etc/nvml-gpu-ha.conf
sudo chown root:root /etc/nvml-gpu-ha.conf
```

### Command Line Options

Command line flags override configuration file settings:

```bash
nvml-gpu-ha [flags]

Flags:
  --config string          Configuration file path (default "/etc/nvml-gpu-ha.conf")
  --hostname string        Hostname prefix for GPU names (default: system hostname)
  --mqtt-host string       MQTT broker host (default "localhost")
  --mqtt-port int          MQTT broker port (default 1883)
  --mqtt-username string   MQTT username
  --mqtt-password string   MQTT password
  --mqtt-lwt-enable        Enable MQTT Last Will and Testament (default true)
  --mqtt-retain            Retain MQTT messages (default true)
  --polling-period int     GPU polling period in seconds (default 30)
  -h, --help              help for nvml-gpu-ha
```

### Example Usage

```bash
# Using configuration file (recommended)
nvml-gpu-ha

# Using custom hostname
nvml-gpu-ha --hostname=MY-SERVER

# Using custom config file location
nvml-gpu-ha --config=/path/to/custom.conf

# Override config file with command line flags
nvml-gpu-ha --mqtt-host=192.168.1.200 --polling-period=10

# Pure command line usage (no config file)
nvml-gpu-ha \
  --hostname=GAMING-PC \
  --mqtt-host=192.168.1.100 \
  --mqtt-port=1883 \
  --mqtt-username=homeassistant \
  --mqtt-password=secretpassword \
  --polling-period=10

# Minimal override  
nvml-gpu-ha --mqtt-host=mqtt.local --hostname=SERVER-01
```

### Configuration Priority

Configuration is loaded in the following order (later sources override earlier ones):

1. **Default values** (built-in defaults)
2. **Configuration file** (`/etc/nvml-gpu-ha.conf` or specified via `--config`)
3. **Command line flags** (highest priority)

This allows you to set base configuration in a file and override specific values via command line as needed.

## Systemd Service Setup

### Create Service File

```bash
# Generate service file
make systemd-service

# Install the service
sudo cp nvml-gpu-ha.service /etc/systemd/system/
sudo systemctl daemon-reload

# Enable and start the service
sudo systemctl enable nvml-gpu-ha
sudo systemctl start nvml-gpu-ha

# Check status
sudo systemctl status nvml-gpu-ha
```

### Manual Service File

Create `/etc/systemd/system/nvml-gpu-ha.service`:

```ini
[Unit]
Description=NVIDIA GPU Monitoring for Home Assistant
After=network.target
Wants=network.target

[Service]
Type=simple
User=nobody
Group=nogroup
ExecStart=/usr/local/bin/nvml-gpu-ha
Restart=always
RestartSec=10

[Install]
WantedBy=multi-user.target
```

**Note**: With configuration file support, the service file is much cleaner. All settings are read from `/etc/nvml-gpu-ha.conf` automatically.

## Docker Usage

### Docker Compose

```yaml
version: '3.8'
services:
  nvml-gpu-ha:
    image: ghcr.io/pccr10001/nvml-gpu-ha:latest
    container_name: nvml-gpu-ha
    restart: unless-stopped
    runtime: nvidia
    environment:
      - NVIDIA_VISIBLE_DEVICES=all
    command: [
      "--mqtt-host=192.168.1.100",
      "--mqtt-username=homeassistant", 
      "--mqtt-password=secretpassword",
      "--polling-period=30"
    ]
    deploy:
      resources:
        reservations:
          devices:
            - driver: nvidia
              count: all
              capabilities: [gpu]
```

### Docker Run

```bash
docker run -d \
  --name nvml-gpu-ha \
  --restart unless-stopped \
  --runtime nvidia \
  -e NVIDIA_VISIBLE_DEVICES=all \
  ghcr.io/pccr10001/nvml-gpu-ha:latest \
  --mqtt-host=192.168.1.100 \
  --mqtt-username=homeassistant \
  --mqtt-password=secretpassword
```

## Home Assistant Configuration

### MQTT Integration

Ensure MQTT integration is configured in Home Assistant:

```yaml
# configuration.yaml
mqtt:
  broker: localhost  # Your MQTT broker
  username: homeassistant
  password: secretpassword
  discovery: true    # Enable MQTT Discovery
```

### Sensor Entities

Once running, sensors will automatically appear in Home Assistant under:

- **Device**: `{HOSTNAME} {PCI ID} - NVIDIA {MODEL} {VRAM}`
- **Entities**:
  - `sensor.{pci_id}_nvidia_{model}_{vram}_power_draw`
  - `sensor.{pci_id}_nvidia_{model}_{vram}_performance_level`
  - `sensor.{pci_id}_nvidia_{model}_{vram}_vram_usage`
  - `sensor.{pci_id}_nvidia_{model}_{vram}_gpu_utilization`
  - `sensor.{pci_id}_nvidia_{model}_{vram}_temperature`

## Development

### Building for Development

```bash
# Install dependencies
make deps

# Build for current platform
make build

# Build for all platforms
make build-linux
make build-linux-arm64
make build-windows

# Run tests
make test
```

### Cross-compilation Notes

- **Linux builds**: Include official NVIDIA go-nvml bindings (production)
- **Windows builds**: Stub implementation for development only
- **C String Handling**: Properly converts NVIDIA's C-style char arrays to Go strings
- The application must run on Linux with NVIDIA drivers for full functionality

## Performance & Reliability

### Request Protection
This version includes several performance improvements:

- **Mutex-based request protection** - Prevents overlapping NVML calls that can cause slowdowns
- **Timeout protection** - GPU metric requests timeout after 10 seconds to prevent hanging
- **Concurrent monitoring** - Multiple GPUs are monitored in parallel for faster updates
- **Smart scheduling** - Skips monitoring cycles if previous requests are still running

### Version Information
The application displays NVML and driver version information at startup for debugging:

```
2024/08/18 11:00:00 Hostname: MY-SERVER
2024/08/18 11:00:00 NVML Version: 12.535.133.00
2024/08/18 11:00:00 NVIDIA Driver Version: 535.133.00
2024/08/18 11:00:00 Found 2 NVIDIA GPU(s)
2024/08/18 11:00:00 GPU 0: GeForce RTX 3080 (00:01:00.0, 10.0GB)
2024/08/18 11:00:00 GPU 1: GeForce RTX 4090 (00:02:00.0, 24.0GB)
```

## Troubleshooting

### Common Issues

1. **"Failed to initialize NVML"**
   - Ensure NVIDIA drivers are installed and up-to-date
   - Check that `/usr/lib/x86_64-linux-gnu/libnvidia-ml.so.1` exists
   - Run with `nvidia-smi` first to verify GPU access

2. **"No NVIDIA GPUs found"**
   - Verify GPUs are detected: `nvidia-smi -L`
   - Check driver compatibility
   - Ensure process has GPU access permissions

3. **MQTT connection issues**
   - Verify broker address and credentials
   - Check firewall settings
   - Test with mosquitto client tools

4. **Sensors not appearing in Home Assistant**
   - Ensure MQTT discovery is enabled
   - Check MQTT broker logs
   - Verify topic structure in MQTT explorer

### Debug Mode

Enable verbose logging by checking the application logs:

```bash
# Systemd logs
sudo journalctl -u nvml-gpu-ha -f

# Docker logs
docker logs -f nvml-gpu-ha
```

### Manual Testing

Test MQTT connectivity:

```bash
# Subscribe to all topics
mosquitto_sub -h YOUR_MQTT_HOST -u YOUR_USERNAME -P YOUR_PASSWORD -t "homeassistant/sensor/nvml-gpu/+"

# Check discovery topics
mosquitto_sub -h YOUR_MQTT_HOST -u YOUR_USERNAME -P YOUR_PASSWORD -t "homeassistant/sensor/nvml-gpu/+/config"
```

## Contributing

1. Fork the repository
2. Create a feature branch
3. Make changes and test
4. Submit a pull request

## License

This project is licensed under the MIT License - see the LICENSE file for details.

## Acknowledgments

- NVIDIA for the NVML API
- Home Assistant community
- Eclipse Paho MQTT Go client
- Cobra CLI framework
