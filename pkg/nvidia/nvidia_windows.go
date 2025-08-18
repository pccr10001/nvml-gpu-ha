//go:build windows
// +build windows

package nvidia

import (
	"errors"
	"fmt"
)

// GPUDevice represents an NVIDIA GPU device
type GPUDevice struct {
	Index    int
	Handle   interface{} // Placeholder for Windows
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
	return errors.New("NVML is not supported on Windows build. Please use Linux build for production")
}

// Shutdown shuts down the NVML library
func Shutdown() error {
	return nil
}

// GetGPUDevices returns all available GPU devices
func GetGPUDevices() ([]GPUDevice, error) {
	return nil, errors.New("NVML is not supported on Windows build. Please use Linux build for production")
}

// GetGPUMetrics retrieves current metrics for a GPU device
func GetGPUMetrics(device GPUDevice) (GPUMetrics, error) {
	return GPUMetrics{}, errors.New("NVML is not supported on Windows build. Please use Linux build for production")
}

// GetDeviceID generates a unique device identifier for MQTT topics
func GetDeviceID(device GPUDevice) string {
	return "mock_device_id"
}

// GetShortPCIBusID formats PCI Bus ID from 00000000:04:00.0 to 00:04:00.0 (Windows stub)
func GetShortPCIBusID(pciBusID string) string {
	return pciBusID
}

// GetDeviceDisplayName generates a display name in the format: {HOSTNAME} {PCI ID} - NVIDIA {MODEL} {VRAM}
func GetDeviceDisplayName(device GPUDevice, hostname string) string {
	return fmt.Sprintf("%s 00:01:00.0 - NVIDIA Mock GPU 0GB", hostname)
}

// GetNVMLVersion returns the NVML version information (Windows stub)
func GetNVMLVersion() (string, error) {
	return "", errors.New("NVML is not supported on Windows build. Please use Linux build for production")
}

// GetDriverVersion returns the NVIDIA driver version (Windows stub)
func GetDriverVersion() (string, error) {
	return "", errors.New("NVML is not supported on Windows build. Please use Linux build for production")
}

// IsDeviceAvailable checks if a GPU device is still available and responsive (Windows stub)
func IsDeviceAvailable(device GPUDevice) bool {
	return false
}
