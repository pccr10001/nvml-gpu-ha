//go:build linux
// +build linux

package nvidia

import (
	"fmt"
	"strings"
	"sync"
	"time"
	"unsafe"

	"github.com/NVIDIA/go-nvml/pkg/nvml"
)

// requestMutex prevents overlapping NVML requests to avoid slowdowns
var requestMutex sync.Mutex

// convertCString converts a C-style char array to a Go string
func convertCString(cstr [32]int8) string {
	n := 0
	for i := 0; i < len(cstr); i++ {
		if cstr[i] == 0 {
			break
		}
		n++
	}
	return string((*[32]byte)(unsafe.Pointer(&cstr[0]))[:n])
}

// GPUDevice represents an NVIDIA GPU device
type GPUDevice struct {
	Index    int
	Handle   nvml.Device
	Name     string
	PCIBusID string
	Memory   uint64 // Total memory in bytes
	UUID     string
}

// GPUMetrics contains current GPU metrics
type GPUMetrics struct {
	PowerDraw         float64 // Watts
	PerformanceLevel  string  // P0, P8, etc.
	MemoryUsage       float64 // Percentage
	GPUUtilization    int     // Percentage
	MemoryUtilization int     // Percentage
	Temperature       int     // Celsius
}

// Init initializes the NVML library
func Init() error {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	ret := nvml.Init()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to initialize NVML: %s", nvml.ErrorString(ret))
	}
	return nil
}

// Shutdown shuts down the NVML library
func Shutdown() error {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	ret := nvml.Shutdown()
	if ret != nvml.SUCCESS {
		return fmt.Errorf("failed to shutdown NVML: %s", nvml.ErrorString(ret))
	}
	return nil
}

// GetGPUDevices returns all available GPU devices
func GetGPUDevices() ([]GPUDevice, error) {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	count, ret := nvml.DeviceGetCount()
	if ret != nvml.SUCCESS {
		return nil, fmt.Errorf("failed to get device count: %s", nvml.ErrorString(ret))
	}

	devices := make([]GPUDevice, count)

	for i := 0; i < count; i++ {
		device, ret := nvml.DeviceGetHandleByIndex(i)
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get device handle for index %d: %s", i, nvml.ErrorString(ret))
		}

		// Get device name
		name, ret := device.GetName()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get device name: %s", nvml.ErrorString(ret))
		}

		// Get PCI Bus ID
		pciInfo, ret := device.GetPciInfo()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get PCI info: %s", nvml.ErrorString(ret))
		}

		// Get memory info
		memInfo, ret := device.GetMemoryInfo()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get memory info: %s", nvml.ErrorString(ret))
		}

		// Get UUID
		uuid, ret := device.GetUUID()
		if ret != nvml.SUCCESS {
			return nil, fmt.Errorf("failed to get device UUID: %s", nvml.ErrorString(ret))
		}

		devices[i] = GPUDevice{
			Index:    i,
			Handle:   device,
			Name:     name,
			PCIBusID: convertCString(pciInfo.BusId),
			Memory:   memInfo.Total,
			UUID:     uuid,
		}
	}

	return devices, nil
}

// GetGPUMetrics retrieves current metrics for a GPU device with timeout protection
func GetGPUMetrics(device GPUDevice) (GPUMetrics, error) {
	// Use a timeout channel to prevent hanging requests
	done := make(chan struct {
		metrics GPUMetrics
		err     error
	}, 1)

	go func() {
		metrics, err := getGPUMetricsInternal(device)
		done <- struct {
			metrics GPUMetrics
			err     error
		}{metrics, err}
	}()

	select {
	case result := <-done:
		return result.metrics, result.err
	case <-time.After(10 * time.Second):
		return GPUMetrics{}, fmt.Errorf("timeout getting GPU metrics for device %s", device.Name)
	}
}

// getGPUMetricsInternal performs the actual NVML calls with mutex protection
func getGPUMetricsInternal(device GPUDevice) (GPUMetrics, error) {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	metrics := GPUMetrics{}

	// Get power draw
	power, ret := device.Handle.GetPowerUsage()
	if ret == nvml.SUCCESS {
		metrics.PowerDraw = float64(power) / 1000.0 // Convert mW to W
	} else if ret != nvml.ERROR_NOT_SUPPORTED {
		return metrics, fmt.Errorf("failed to get power usage: %s", nvml.ErrorString(ret))
	}

	// Get performance state
	perfState, ret := device.Handle.GetPerformanceState()
	if ret == nvml.SUCCESS {
		metrics.PerformanceLevel = fmt.Sprintf("P%d", int(perfState))
	} else if ret != nvml.ERROR_NOT_SUPPORTED {
		return metrics, fmt.Errorf("failed to get performance state: %s", nvml.ErrorString(ret))
	}

	// Get memory usage
	memInfo, ret := device.Handle.GetMemoryInfo()
	if ret == nvml.SUCCESS {
		metrics.MemoryUsage = float64(memInfo.Used) / float64(memInfo.Total) * 100.0
	} else {
		return metrics, fmt.Errorf("failed to get memory info: %s", nvml.ErrorString(ret))
	}

	// Get utilization rates
	utilization, ret := device.Handle.GetUtilizationRates()
	if ret == nvml.SUCCESS {
		metrics.GPUUtilization = int(utilization.Gpu)
		metrics.MemoryUtilization = int(utilization.Memory)
	} else if ret != nvml.ERROR_NOT_SUPPORTED {
		return metrics, fmt.Errorf("failed to get utilization rates: %s", nvml.ErrorString(ret))
	}

	// Get temperature
	temperature, ret := device.Handle.GetTemperature(nvml.TEMPERATURE_GPU)
	if ret == nvml.SUCCESS {
		metrics.Temperature = int(temperature)
	} else if ret != nvml.ERROR_NOT_SUPPORTED {
		return metrics, fmt.Errorf("failed to get temperature: %s", nvml.ErrorString(ret))
	}

	return metrics, nil
}

// GetShortPCIBusID formats PCI Bus ID from 00000000:04:00.0 to 00:04:00.0
func GetShortPCIBusID(pciBusID string) string {
	// Split by colon to separate domain:bus:device.function
	parts := strings.Split(pciBusID, ":")
	if len(parts) >= 3 {
		// Format: domain should be 2 digits instead of 8
		domain := parts[0]
		if len(domain) > 2 {
			// Take the last 2 characters of the domain
			domain = domain[len(domain)-2:]
		}
		// Return formatted as domain:bus:device.function
		return fmt.Sprintf("%s:%s:%s", domain, parts[1], parts[2])
	}
	// If format is unexpected, return as-is
	return pciBusID
}

// GetDeviceID generates a unique device identifier for MQTT topics
func GetDeviceID(device GPUDevice) string {
	// Format PCI Bus ID to short format and remove unwanted characters
	shortPCIBusID := GetShortPCIBusID(device.PCIBusID)
	deviceID := strings.Replace(shortPCIBusID, ":", "_", -1)
	deviceID = strings.Replace(deviceID, ".", "_", -1)

	// Add GPU UUID suffix (first 8 characters) to ensure uniqueness across different machines
	uuidSuffix := strings.Replace(device.UUID, "-", "", -1)
	if len(uuidSuffix) > 8 {
		uuidSuffix = uuidSuffix[:8]
	}

	return strings.ToLower(fmt.Sprintf("%s_%s", deviceID, uuidSuffix))
}

// GetDeviceDisplayName generates a display name in the format: {HOSTNAME} {PCI ID} - NVIDIA {MODEL} {VRAM}
func GetDeviceDisplayName(device GPUDevice, hostname string) string {
	vramGB := float64(device.Memory) / (1024 * 1024 * 1024)

	// Extract model name from device name (remove "NVIDIA" prefix if present)
	modelName := device.Name
	if strings.HasPrefix(strings.ToUpper(modelName), "NVIDIA ") {
		modelName = strings.TrimPrefix(modelName, "NVIDIA ")
	}

	// Use short format PCI Bus ID (00:04:00.0 instead of 00000000:04:00.0)
	shortPCIBusID := GetShortPCIBusID(device.PCIBusID)
	return fmt.Sprintf("%s %s - NVIDIA %s %.0fGB", hostname, shortPCIBusID, modelName, vramGB)
}

// GetNVMLVersion returns the NVML version information
func GetNVMLVersion() (string, error) {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	version, ret := nvml.SystemGetNVMLVersion()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("failed to get NVML version: %s", nvml.ErrorString(ret))
	}
	return version, nil
}

// GetDriverVersion returns the NVIDIA driver version
func GetDriverVersion() (string, error) {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	version, ret := nvml.SystemGetDriverVersion()
	if ret != nvml.SUCCESS {
		return "", fmt.Errorf("failed to get driver version: %s", nvml.ErrorString(ret))
	}
	return version, nil
}

// IsDeviceAvailable checks if a GPU device is still available and responsive
func IsDeviceAvailable(device GPUDevice) bool {
	requestMutex.Lock()
	defer requestMutex.Unlock()

	// Try to get device name as a simple health check
	_, ret := device.Handle.GetName()
	return ret == nvml.SUCCESS
}
