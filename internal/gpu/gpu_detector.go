package gpu

import (
	"fmt"
	"os"
	"os/exec"
	"strings"

	"go.uber.org/zap"
)

// GPUDetector handles GPU detection and configuration
type GPUDetector struct {
	logger *zap.Logger
}

// GPUInfo contains information about available GPU devices
type GPUInfo struct {
	Available     bool
	DeviceCount   int
	DeviceName    string
	CUDAVersion   string
	DriverVersion string
}

// NewGPUDetector creates a new GPU detector instance
func NewGPUDetector(logger *zap.Logger) *GPUDetector {
	return &GPUDetector{
		logger: logger,
	}
}

// DetectGPU detects available NVIDIA GPU devices
func (g *GPUDetector) DetectGPU() (*GPUInfo, error) {
	gpuInfo := &GPUInfo{
		Available:     false,
		DeviceCount:   0,
		DeviceName:    "",
		CUDAVersion:   "",
		DriverVersion: "",
	}

	// Try to detect GPU using nvidia-smi
	if err := g.detectWithNvidiaSMI(gpuInfo); err != nil {
		g.logger.Debug("nvidia-smi detection failed", zap.Error(err))
		// Fallback to checking CUDA environment
		if err := g.detectWithCUDAEnv(gpuInfo); err != nil {
			g.logger.Debug("CUDA environment detection failed", zap.Error(err))
			// Try checking for CUDA toolkit installation
			if err := g.detectWithCUDAToolkit(gpuInfo); err != nil {
				g.logger.Debug("CUDA toolkit detection failed", zap.Error(err))
				return gpuInfo, nil // Return with Available=false
			}
		}
	}

	g.logger.Info("GPU detection completed",
		zap.Bool("available", gpuInfo.Available),
		zap.Int("device_count", gpuInfo.DeviceCount),
		zap.String("device_name", gpuInfo.DeviceName),
		zap.String("cuda_version", gpuInfo.CUDAVersion))

	return gpuInfo, nil
}

// detectWithNvidiaSMI attempts to detect GPU using nvidia-smi command
func (g *GPUDetector) detectWithNvidiaSMI(gpuInfo *GPUInfo) error {
	// First, check how many GPUs are available using --list-gpus
	countCmd := exec.Command("nvidia-smi", "--list-gpus")
	countOutput, err := countCmd.Output()
	if err != nil {
		return fmt.Errorf("nvidia-smi command failed: %w", err)
	}

	// Count the number of lines (each line represents one GPU)
	lines := strings.Split(strings.TrimSpace(string(countOutput)), "\n")
	if len(lines) == 0 || (len(lines) == 1 && lines[0] == "") {
		return fmt.Errorf("no GPUs found by nvidia-smi")
	}

	deviceCount := len(lines)

	// Now get detailed information about the first GPU
	infoCmd := exec.Command("nvidia-smi", "--query-gpu=name,driver_version", "--format=csv,noheader,nounits", "--id=0")
	infoOutput, err := infoCmd.Output()
	if err != nil {
		return fmt.Errorf("nvidia-smi info query failed: %w", err)
	}

	// Parse GPU info: format should be "name,driver_version"
	infoLines := strings.Split(strings.TrimSpace(string(infoOutput)), "\n")
	if len(infoLines) == 0 {
		return fmt.Errorf("no GPU info from nvidia-smi")
	}

	// Parse first line
	parts := strings.Split(infoLines[0], ",")
	if len(parts) < 2 {
		return fmt.Errorf("unexpected nvidia-smi info format: %s", infoLines[0])
	}

	gpuInfo.DeviceCount = deviceCount
	gpuInfo.DeviceName = strings.TrimSpace(parts[0])
	gpuInfo.DriverVersion = strings.TrimSpace(parts[1])

	// Get CUDA version from environment or runtime
	if cudaVersion := os.Getenv("CUDA_VERSION"); cudaVersion != "" {
		gpuInfo.CUDAVersion = cudaVersion
	} else {
		// Default CUDA version based on container environment
		gpuInfo.CUDAVersion = "12.4.0"
	}

	gpuInfo.Available = deviceCount > 0

	// Log detailed GPU information when available
	if gpuInfo.Available {
		g.logger.Info("GPU auto-detection successful",
			zap.Int("device_count", deviceCount),
			zap.String("device_name", gpuInfo.DeviceName),
			zap.String("driver_version", gpuInfo.DriverVersion),
			zap.String("cuda_version", gpuInfo.CUDAVersion))
	}

	return nil
}

// detectWithCUDAEnv attempts to detect GPU using CUDA environment variables
func (g *GPUDetector) detectWithCUDAEnv(gpuInfo *GPUInfo) error {
	// Check for CUDA environment variables
	cudaPath := os.Getenv("CUDA_PATH")
	cudaVersion := os.Getenv("CUDA_VERSION")
	visibleDevices := os.Getenv("CUDA_VISIBLE_DEVICES")

	if cudaPath == "" && cudaVersion == "" && visibleDevices == "" {
		return fmt.Errorf("no CUDA environment variables found")
	}

	// If CUDA_VISIBLE_DEVICES is set, parse it
	if visibleDevices != "" {
		if visibleDevices == "-1" {
			// No devices visible
			return nil
		}

		devices := strings.Split(visibleDevices, ",")
		gpuInfo.DeviceCount = len(devices)
		gpuInfo.Available = gpuInfo.DeviceCount > 0
	}

	// Set CUDA version from environment if available
	if cudaVersion != "" {
		gpuInfo.CUDAVersion = cudaVersion
	}

	return nil
}

// detectWithCUDAToolkit attempts to detect CUDA toolkit installation
func (g *GPUDetector) detectWithCUDAToolkit(gpuInfo *GPUInfo) error {
	// Check for common CUDA library paths
	cudaPaths := []string{
		"/usr/local/cuda",
		"/opt/cuda",
		"/usr/cuda",
	}

	for _, path := range cudaPaths {
		if _, err := os.Stat(path); err == nil {
			// Check for CUDA version file
			versionFile := path + "/version.txt"
			if versionData, err := os.ReadFile(versionFile); err == nil {
				lines := strings.Split(string(versionData), "\n")
				for _, line := range lines {
					if strings.Contains(line, "CUDA Version") {
						parts := strings.Split(line, " ")
						if len(parts) >= 3 {
							gpuInfo.CUDAVersion = strings.TrimSpace(parts[2])
							gpuInfo.Available = true
							gpuInfo.DeviceCount = 1 // Assume at least one device
							return nil
						}
					}
				}
			}

			// Fallback: assume CUDA 12.4 if directory exists
			gpuInfo.CUDAVersion = "12.4"
			gpuInfo.Available = true
			gpuInfo.DeviceCount = 1
			return nil
		}
	}

	return fmt.Errorf("CUDA toolkit not found in standard locations")
}

// IsCUDAAvailable checks if CUDA is available and can be used
func (g *GPUDetector) IsCUDAAvailable() bool {
	gpuInfo, err := g.DetectGPU()
	if err != nil {
		g.logger.Debug("GPU detection failed", zap.Error(err))
		return false
	}
	return gpuInfo.Available
}

// GetOptimalDeviceID returns the optimal GPU device ID to use
func (g *GPUDetector) GetOptimalDeviceID(configuredID int) int {
	gpuInfo, err := g.DetectGPU()
	if err != nil || !gpuInfo.Available {
		return -1 // No GPU available
	}

	// If configured ID is within valid range, use it
	if configuredID >= 0 && configuredID < gpuInfo.DeviceCount {
		return configuredID
	}

	// Otherwise use first available GPU
	return 0
}

// GetGPUInfo returns detailed GPU information
func (g *GPUDetector) GetGPUInfo() *GPUInfo {
	gpuInfo, err := g.DetectGPU()
	if err != nil {
		g.logger.Debug("Failed to get GPU info", zap.Error(err))
		return &GPUInfo{Available: false}
	}
	return gpuInfo
}
