package main

import (
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/spf13/cobra"
)

var (
	mqttHost     string
	mqttPort     int
	mqttUsername string
	mqttPassword string
	tempDir      string
	deviceID     string

	rootCmd = &cobra.Command{
		Use:   "ha-gpu-ccd",
		Short: "Home Assistant GPU CCD Temperature Monitor",
		Long:  "Monitor GPU temperatures from Home Assistant MQTT and write to sysfs format files for CCD integration",
		Run:   run,
	}
)

func init() {
	rootCmd.PersistentFlags().StringVar(&mqttHost, "mqtt-host", "localhost", "MQTT broker host")
	rootCmd.PersistentFlags().IntVar(&mqttPort, "mqtt-port", 1883, "MQTT broker port")
	rootCmd.PersistentFlags().StringVar(&mqttUsername, "mqtt-username", "", "MQTT username")
	rootCmd.PersistentFlags().StringVar(&mqttPassword, "mqtt-password", "", "MQTT password")
	rootCmd.PersistentFlags().StringVar(&tempDir, "temp-dir", "/tmp", "Directory to write temperature files")
	rootCmd.PersistentFlags().StringVar(&deviceID, "device-id", "", "Specific GPU device ID to monitor (leave empty to monitor all devices)")
}

func main() {
	if err := rootCmd.Execute(); err != nil {
		log.Fatal(err)
	}
}

func run(cmd *cobra.Command, args []string) {
	log.Printf("Starting ha-gpu-ccd")
	log.Printf("MQTT Broker: %s:%d", mqttHost, mqttPort)
	log.Printf("Temperature directory: %s", tempDir)
	log.Printf("MQTT Username: %s", func() string {
		if mqttUsername != "" {
			return mqttUsername
		}
		return "(none)"
	}())

	// Create temp directory if it doesn't exist
	if err := os.MkdirAll(tempDir, 0755); err != nil {
		log.Fatalf("Failed to create temp directory %s: %v", tempDir, err)
	}

	// Setup MQTT client
	mqttClient := setupMQTTClient()
	defer mqttClient.Disconnect(250)

	// Setup graceful shutdown
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)

	log.Println("Monitoring GPU temperatures from Home Assistant...")
	log.Println("Press Ctrl+C to stop")

	<-quit
	log.Println("Shutting down...")
}

func setupMQTTClient() mqtt.Client {
	opts := mqtt.NewClientOptions()
	opts.AddBroker(fmt.Sprintf("tcp://%s:%d", mqttHost, mqttPort))

	// Generate random suffix for client ID to avoid conflicts
	randomBytes := make([]byte, 3)
	if _, err := rand.Read(randomBytes); err == nil {
		clientID := fmt.Sprintf("ha-gpu-ccd-%s", hex.EncodeToString(randomBytes))
		opts.SetClientID(clientID)
	} else {
		// Fallback to timestamp if random generation fails
		clientID := fmt.Sprintf("ha-gpu-ccd-%d", time.Now().Unix())
		opts.SetClientID(clientID)
	}

	opts.SetUsername(mqttUsername)
	opts.SetPassword(mqttPassword)
	opts.SetAutoReconnect(true)
	opts.SetConnectRetry(true)
	opts.SetConnectRetryInterval(10 * time.Second)
	opts.SetKeepAlive(60 * time.Second)

	opts.SetOnConnectHandler(func(client mqtt.Client) {
		log.Println("Connected to MQTT broker")

		// Wait a moment for connection to stabilize
		go func() {
			time.Sleep(1 * time.Second)

			// Check if connection is still active
			if !client.IsConnected() {
				log.Println("Connection lost during stabilization")
				return
			}

			// Subscribe to temperature topics after successful connection
			if err := subscribeToTemperatureTopics(client); err != nil {
				log.Printf("Failed to subscribe to temperature topics: %v", err)
			}
		}()
	})

	opts.SetConnectionLostHandler(func(client mqtt.Client, err error) {
		log.Printf("Connection lost to MQTT broker: %v", err)
	})

	client := mqtt.NewClient(opts)
	if token := client.Connect(); token.Wait() && token.Error() != nil {
		log.Fatalf("Failed to connect to MQTT broker: %v", token.Error())
	}

	// Wait a bit to ensure connection is stable
	time.Sleep(2 * time.Second)

	return client
}

func subscribeToTemperatureTopics(client mqtt.Client) error {
	var topic string

	if deviceID != "" {
		// Subscribe to specific device temperature topic
		topic = fmt.Sprintf("homeassistant/sensor/nvml-gpu/%s_temperature/state", deviceID)
	} else {
		// Subscribe to all GPU temperature topics using # wildcard
		topic = "homeassistant/sensor/nvml-gpu/#"
	}

	// Wait for subscription with timeout
	token := client.Subscribe(topic, 1, onTemperatureMessage)
	if !token.WaitTimeout(10 * time.Second) {
		return fmt.Errorf("timeout waiting for subscription to %s", topic)
	}

	if token.Error() != nil {
		return fmt.Errorf("failed to subscribe to %s: %v", topic, token.Error())
	}

	log.Printf("Successfully subscribed to: %s", topic)
	return nil
}

func onTemperatureMessage(client mqtt.Client, msg mqtt.Message) {
	topic := msg.Topic()
	payload := string(msg.Payload())

	// Filter for temperature topics only
	// Topic format: homeassistant/sensor/nvml-gpu/{DEVICEID}_temperature/state
	if !strings.Contains(topic, "_temperature/state") {
		// Ignore non-temperature topics
		return
	}

	parts := strings.Split(topic, "/")
	if len(parts) < 4 {
		log.Printf("Invalid topic format: %s", topic)
		return
	}

	deviceSensor := parts[3] // This should be {DEVICEID}_temperature
	if !strings.HasSuffix(deviceSensor, "_temperature") {
		log.Printf("Topic does not end with _temperature: %s", topic)
		return
	}

	deviceID := strings.TrimSuffix(deviceSensor, "_temperature")

	// Parse temperature from JSON payload
	var temperature float64
	if err := json.Unmarshal([]byte(payload), &temperature); err != nil {
		log.Printf("Failed to parse temperature from payload '%s': %v", payload, err)
		return
	}

	log.Printf("Received temperature for device %s: %.1f°C", deviceID, temperature)

	// Convert temperature to sysfs format (millidegrees)
	// Example: 80.5°C -> 80500
	tempMillidegrees := int(temperature * 1000)

	// Write to temp file
	tempFile := filepath.Join(tempDir, fmt.Sprintf("temp_%s", deviceID))
	if err := writeTemperatureFile(tempFile, tempMillidegrees); err != nil {
		log.Printf("Failed to write temperature file %s: %v", tempFile, err)
		return
	}

	log.Printf("Updated %s: %d (%.1f°C)", tempFile, tempMillidegrees, temperature)
}

func writeTemperatureFile(filename string, tempMillidegrees int) error {
	content := strconv.Itoa(tempMillidegrees)

	// Create or overwrite the file
	file, err := os.Create(filename)
	if err != nil {
		return fmt.Errorf("failed to create file: %v", err)
	}
	defer file.Close()

	// Write the temperature value
	if _, err := file.WriteString(content); err != nil {
		return fmt.Errorf("failed to write temperature: %v", err)
	}

	return nil
}
