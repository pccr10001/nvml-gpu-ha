package homeassistant

import (
	"encoding/json"
	"fmt"
	"log"

	mqtt "github.com/eclipse/paho.mqtt.golang"
	"github.com/pccr10001/nvml-gpu-ha/pkg/config"
	"github.com/pccr10001/nvml-gpu-ha/pkg/nvidia"
)

// Manager handles Home Assistant MQTT Discovery
type Manager struct {
	client mqtt.Client
	config *config.Config
}

// SensorConfig represents Home Assistant sensor configuration
type SensorConfig struct {
	Name                string      `json:"name"`
	StateTopic          string      `json:"state_topic"`
	UniqueID            string      `json:"unique_id"`
	DeviceClass         string      `json:"device_class,omitempty"`
	UnitOfMeasurement   string      `json:"unit_of_measurement,omitempty"`
	Icon                string      `json:"icon,omitempty"`
	Device              *DeviceInfo `json:"device"`
	AvailabilityTopic   string      `json:"availability_topic,omitempty"`
	PayloadAvailable    string      `json:"payload_available,omitempty"`
	PayloadNotAvailable string      `json:"payload_not_available,omitempty"`
	ValueTemplate       string      `json:"value_template,omitempty"`
	StateClass          string      `json:"state_class,omitempty"`
	ForceUpdate         bool        `json:"force_update,omitempty"`
}

// DeviceInfo represents device information for Home Assistant
type DeviceInfo struct {
	Identifiers  []string `json:"identifiers"`
	Name         string   `json:"name"`
	Model        string   `json:"model"`
	Manufacturer string   `json:"manufacturer"`
	SwVersion    string   `json:"sw_version,omitempty"`
}

// NewManager creates a new Home Assistant discovery manager
func NewManager(client mqtt.Client, config *config.Config) *Manager {
	return &Manager{
		client: client,
		config: config,
	}
}

// RegisterGPUSensors registers all sensors for a GPU device
func (m *Manager) RegisterGPUSensors(device nvidia.GPUDevice, hostname string) error {
	deviceID := nvidia.GetDeviceID(device)
	deviceName := nvidia.GetDeviceDisplayName(device, hostname)

	deviceInfo := &DeviceInfo{
		Identifiers:  []string{deviceID, device.UUID},
		Name:         deviceName,
		Model:        device.Name,
		Manufacturer: "NVIDIA",
		SwVersion:    "NVML",
	}

	sensors := []struct {
		key         string
		name        string
		deviceClass string
		unit        string
		icon        string
		stateClass  string
		template    string
	}{
		{
			key:         "power_draw",
			name:        "Power Draw",
			deviceClass: "power",
			unit:        "W",
			icon:        "mdi:lightning-bolt",
			stateClass:  "measurement",
		},
		{
			key:         "performance_level",
			name:        "Performance Level",
			deviceClass: "",
			unit:        "",
			icon:        "mdi:speedometer",
			stateClass:  "",
		},
		{
			key:         "memory_usage",
			name:        "VRAM Usage",
			deviceClass: "",
			unit:        "%",
			icon:        "mdi:memory",
			stateClass:  "measurement",
			template:    "{{ value | round(1) }}",
		},
		{
			key:         "gpu_utilization",
			name:        "GPU Utilization",
			deviceClass: "",
			unit:        "%",
			icon:        "mdi:chip",
			stateClass:  "measurement",
		},
		{
			key:         "temperature",
			name:        "GPU Temperature",
			deviceClass: "temperature",
			unit:        "°C",
			icon:        "mdi:thermometer",
			stateClass:  "measurement",
		},
	}

	for _, sensor := range sensors {
		if err := m.registerSensor(deviceID, deviceName, sensor.key, sensor.name,
			sensor.deviceClass, sensor.unit, sensor.icon, sensor.stateClass,
			sensor.template, deviceInfo); err != nil {
			return fmt.Errorf("failed to register sensor %s: %v", sensor.key, err)
		}
	}

	return nil
}

// registerSensor registers a single sensor with Home Assistant
func (m *Manager) registerSensor(deviceID, deviceName, sensorKey, sensorName, deviceClass, unit, icon, stateClass, template string, deviceInfo *DeviceInfo) error {
	uniqueID := fmt.Sprintf("nvml_gpu_%s_%s", deviceID, sensorKey)
	stateTopic := fmt.Sprintf("homeassistant/sensor/nvml-gpu/%s_%s/state", deviceID, sensorKey)
	configTopic := fmt.Sprintf("homeassistant/sensor/nvml-gpu/%s_%s/config", deviceID, sensorKey)

	fullSensorName := sensorName

	sensorConfig := SensorConfig{
		Name:              fullSensorName,
		StateTopic:        stateTopic,
		UniqueID:          uniqueID,
		DeviceClass:       deviceClass,
		UnitOfMeasurement: unit,
		Icon:              icon,
		Device:            deviceInfo,
		StateClass:        stateClass,
		ForceUpdate:       true,
	}

	if template != "" {
		sensorConfig.ValueTemplate = template
	}

	// Add availability if LWT is enabled
	if m.config.MQTTLWTEnable {
		sensorConfig.AvailabilityTopic = "homeassistant/sensor/nvml-gpu-ha/availability"
		sensorConfig.PayloadAvailable = "online"
		sensorConfig.PayloadNotAvailable = "offline"
	}

	configJSON, err := json.Marshal(sensorConfig)
	if err != nil {
		return fmt.Errorf("failed to marshal sensor config: %v", err)
	}

	token := m.client.Publish(configTopic, 1, m.config.MQTTRetain, configJSON)
	if !token.WaitTimeout(5*1e9) || token.Error() != nil { // 5 seconds
		return fmt.Errorf("failed to publish sensor config: %v", token.Error())
	}

	log.Printf("Registered sensor: %s", fullSensorName)
	return nil
}

// RemoveGPUSensors removes all sensors for a GPU device
func (m *Manager) RemoveGPUSensors(device nvidia.GPUDevice) error {
	deviceID := nvidia.GetDeviceID(device)

	sensors := []string{"power_draw", "performance_level", "memory_usage", "gpu_utilization", "temperature"}

	for _, sensor := range sensors {
		configTopic := fmt.Sprintf("homeassistant/sensor/nvml-gpu/%s_%s/config", deviceID, sensor)

		// Send empty payload to remove the sensor
		token := m.client.Publish(configTopic, 1, m.config.MQTTRetain, "")
		if !token.WaitTimeout(5*1e9) || token.Error() != nil {
			log.Printf("Failed to remove sensor %s: %v", sensor, token.Error())
		}
	}

	return nil
}

// PublishAvailability publishes availability status
func (m *Manager) PublishAvailability(status string) error {
	if !m.config.MQTTLWTEnable {
		return nil
	}

	topic := "homeassistant/sensor/nvml-gpu-ha/availability"
	token := m.client.Publish(topic, 1, m.config.MQTTRetain, status)
	if !token.WaitTimeout(5*1e9) || token.Error() != nil {
		return fmt.Errorf("failed to publish availability: %v", token.Error())
	}

	return nil
}
