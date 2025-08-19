package main

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pccr10001/nvml-gpu-ha/pkg/config"
	"github.com/pccr10001/nvml-gpu-ha/pkg/homeassistant"
	"github.com/pccr10001/nvml-gpu-ha/pkg/nvidia"
	"github.com/spf13/cobra"
)

var (
	cfg             *config.Config
	monitoringMutex sync.Mutex
	isMonitoring    bool
	lastMonitorTime time.Time
	rootCmd         = &cobra.Command{
		Use:   "nvml-gpu-ha",
		Short: "NVIDIA GPU monitoring for Home Assistant via MQTT",
		Long:  "Monitor NVIDIA GPU metrics and send them to Home Assistant via MQTT with auto-discovery support",
		Run:   run,
	}
)

func init() {
	// Command line flags
	rootCmd.PersistentFlags().String("config", "/etc/nvml-gpu-ha.conf", "Configuration file path")
	rootCmd.PersistentFlags().String("hostname", "", "Hostname prefix for GPU names (default: system hostname)")
	rootCmd.PersistentFlags().String("mqtt-host", "localhost", "MQTT broker host")
	rootCmd.PersistentFlags().Int("mqtt-port", 1883, "MQTT broker port")
	rootCmd.PersistentFlags().String("mqtt-username", "", "MQTT username")
	rootCmd.PersistentFlags().String("mqtt-password", "", "MQTT password")
	rootCmd.PersistentFlags().Bool("mqtt-lwt-enable", true, "Enable MQTT Last Will and Testament")
	rootCmd.PersistentFlags().Bool("mqtt-retain", true, "Retain MQTT messages")
	rootCmd.PersistentFlags().Int("polling-period", 30, "GPU polling period in seconds")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) {
	var err error
	cfg, err = config.LoadConfig(cmd)
	if err != nil {
		log.Fatal("Failed to load configuration:", err)
	}

	// If hostname is not provided, use system hostname
	if cfg.Hostname == "" {
		if hostname, err := os.Hostname(); err == nil {
			cfg.Hostname = hostname
		} else {
			log.Printf("Warning: Failed to get system hostname, using 'localhost': %v", err)
			cfg.Hostname = "localhost"
		}
	}

	// Display configuration source
	configFile, _ := cmd.Flags().GetString("config")
	if _, err := os.Stat(configFile); err == nil {
		log.Printf("Loaded configuration from: %s", configFile)
	} else {
		log.Printf("Using default configuration (no config file found at: %s)", configFile)
	}

	// Display key configuration values (without sensitive data)
	log.Printf("Hostname: %s", cfg.Hostname)
	log.Printf("MQTT Broker: %s:%d", cfg.MQTTHost, cfg.MQTTPort)
	log.Printf("MQTT Username: %s", func() string {
		if cfg.MQTTUsername != "" {
			return cfg.MQTTUsername
		} else {
			return "(none)"
		}
	}())
	log.Printf("Polling Period: %d seconds", cfg.PollingPeriod)
	log.Printf("MQTT LWT Enabled: %v", cfg.MQTTLWTEnable)
	log.Printf("MQTT Retain: %v", cfg.MQTTRetain)

	// Initialize NVIDIA management library
	if err := nvidia.Init(); err != nil {
		log.Fatal("Failed to initialize NVIDIA management library:", err)
	}
	defer nvidia.Shutdown()

	// Display version information
	if nvmlVersion, err := nvidia.GetNVMLVersion(); err == nil {
		log.Printf("NVML Version: %s", nvmlVersion)
	}
	if driverVersion, err := nvidia.GetDriverVersion(); err == nil {
		log.Printf("NVIDIA Driver Version: %s", driverVersion)
	}

	// Get GPU information
	gpus, err := nvidia.GetGPUDevices()
	if err != nil {
		log.Fatal("Failed to get GPU devices:", err)
	}

	if len(gpus) == 0 {
		log.Fatal("No NVIDIA GPUs found")
	}

	log.Printf("Found %d NVIDIA GPU(s)", len(gpus))
	for i, gpu := range gpus {
		shortPCIID := nvidia.GetShortPCIBusID(gpu.PCIBusID)
		log.Printf("GPU %d: %s (%s, %.1fGB)", i, gpu.Name, shortPCIID, float64(gpu.Memory)/(1024*1024*1024))
	}

	// Setup MQTT client
	mqttClient := setupMQTTClient()
	defer mqttClient.Disconnect(250)

	// Setup Home Assistant discovery
	haManager := homeassistant.NewManager(mqttClient, cfg)

	// Register all GPU sensors with Home Assistant
	for _, gpu := range gpus {
		if err := haManager.RegisterGPUSensors(gpu, cfg.Hostname); err != nil {
			log.Printf("Failed to register sensors for GPU %s: %v", gpu.Name, err)
		}
	}

	// Setup graceful shutdown
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	go func() {
		sigChan := make(chan os.Signal, 1)
		signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
		<-sigChan
		log.Println("Received shutdown signal, stopping...")
		cancel()
	}()

	// Main monitoring loop
	ticker := time.NewTicker(time.Duration(cfg.PollingPeriod) * time.Second)
	defer ticker.Stop()

	log.Printf("Starting GPU monitoring loop (polling every %d seconds)", cfg.PollingPeriod)

	for {
		select {
		case <-ctx.Done():
			log.Println("Shutting down...")
			return
		case <-ticker.C:
			monitorGPUs(mqttClient, gpus)
		}
	}
}

func setupMQTTClient() mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", cfg.MQTTHost, cfg.MQTTPort))

	// Generate random suffix for client ID to avoid conflicts
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err == nil {
		clientID := fmt.Sprintf("nvml-gpu-ha-%s", hex.EncodeToString(randomBytes))
		opts.SetClientID(clientID)
	} else {
		// Fallback to timestamp if random generation fails
		clientID := fmt.Sprintf("nvml-gpu-ha-%d", time.Now().Unix())
		opts.SetClientID(clientID)
	}

	opts.SetUsername(cfg.MQTTUsername)
	opts.SetPassword(cfg.MQTTPassword)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(10 * time.Second)

	if cfg.MQTTLWTEnable {
		opts.SetWill("homeassistant/sensor/nvml-gpu-ha/availability", "offline", 1, cfg.MQTTRetain)
	}

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Println("Connected to MQTT broker")
		if cfg.MQTTLWTEnable {
			client.Publish("homeassistant/sensor/nvml-gpu-ha/availability", 1, cfg.MQTTRetain, "online")
		}
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("Connection lost to MQTT broker: %v", err)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatal("Failed to connect to MQTT broker:", token.Error())
	}

	return client
}

func monitorGPUs(client mqtt.Client, gpus []nvidia.GPUDevice) {
	// Prevent overlapping monitoring requests
	monitoringMutex.Lock()
	defer monitoringMutex.Unlock()

	if isMonitoring {
		log.Printf("Previous monitoring request still in progress, skipping this cycle")
		return
	}

	// Check if enough time has passed since last monitoring
	if time.Since(lastMonitorTime) < time.Duration(cfg.PollingPeriod/2)*time.Second {
		log.Printf("Too soon since last monitoring, skipping this cycle")
		return
	}

	isMonitoring = true
	defer func() {
		isMonitoring = false
		lastMonitorTime = time.Now()
	}()

	log.Printf("Starting GPU monitoring cycle...")
	startTime := time.Now()

	var wg sync.WaitGroup
	for _, gpu := range gpus {
		wg.Add(1)
		go func(gpu nvidia.GPUDevice) {
			defer wg.Done()

			metrics, err := nvidia.GetGPUMetrics(gpu)
			if err != nil {
				log.Printf("Failed to get metrics for GPU %s: %v", gpu.Name, err)
				return
			}

			publishMetrics(client, gpu, metrics)
		}(gpu)
	}

	wg.Wait()
	duration := time.Since(startTime)
	log.Printf("GPU monitoring cycle completed in %v", duration)
}

func publishMetrics(client mqtt.Client, gpu nvidia.GPUDevice, metrics nvidia.GPUMetrics) {
	// Publish individual sensor values
	sensors := map[string]interface{}{
		"power_draw":        metrics.PowerDraw,
		"performance_level": metrics.PerformanceLevel,
		"memory_usage":      metrics.MemoryUsage,
		"gpu_utilization":   metrics.GPUUtilization,
		"temperature":       metrics.Temperature,
	}

	deviceID := nvidia.GetDeviceID(gpu)

	for sensor, value := range sensors {
		topic := fmt.Sprintf("homeassistant/sensor/nvml-gpu/%s_%s/state", deviceID, sensor)

		payload, err := json.Marshal(value)
		if err != nil {
			log.Printf("Failed to marshal sensor data for %s: %v", sensor, err)
			continue
		}

		token := client.Publish(topic, 1, cfg.MQTTRetain, payload)
		if !token.WaitTimeout(5*time.Second) || token.Error() != nil {
			log.Printf("Failed to publish %s data: %v", sensor, token.Error())
		}
	}

	log.Printf("Published metrics for GPU: %s", gpu.Name)
}
