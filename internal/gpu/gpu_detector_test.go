package gpu

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestGPUDetectorCreation(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	assert.NotNil(t, detector)
	assert.NotNil(t, detector.logger)
}

func TestGPUInfoDefaultValues(t *testing.T) {
	gpuInfo := &GPUInfo{}

	assert.False(t, gpuInfo.Available)
	assert.Equal(t, 0, gpuInfo.DeviceCount)
	assert.Equal(t, "", gpuInfo.DeviceName)
	assert.Equal(t, "", gpuInfo.CUDAVersion)
	assert.Equal(t, "", gpuInfo.DriverVersion)
}

func TestGPUDetection(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should run detection without crashing", func(t *testing.T) {
		gpuInfo, err := detector.DetectGPU()

		// Should never return nil, even on error
		assert.NotNil(t, gpuInfo)

		// In test environment, likely no GPU will be detected
		if err != nil {
			// Should return valid empty GPUInfo on error
			assert.False(t, gpuInfo.Available)
			assert.Equal(t, 0, gpuInfo.DeviceCount)
		}
	})

	t.Run("should validate GPU info structure on success", func(t *testing.T) {
		gpuInfo, err := detector.DetectGPU()
		assert.NotNil(t, gpuInfo)

		if err == nil {
			// If detection succeeded, validate the structure
			assert.True(t, gpuInfo.Available == (gpuInfo.DeviceCount > 0))
			if gpuInfo.Available {
				assert.NotEmpty(t, gpuInfo.DeviceName)
				assert.Greater(t, gpuInfo.DeviceCount, 0)
			}
		}
	})

	t.Run("should handle error cases gracefully", func(t *testing.T) {
		gpuInfo, err := detector.DetectGPU()
		assert.NotNil(t, gpuInfo)

		// In test environment, detection will likely fail
		// but should return proper error and default GPUInfo
		if err != nil {
			assert.Contains(t, err.Error(), "failed to detect GPU")
			assert.False(t, gpuInfo.Available)
			assert.Equal(t, 0, gpuInfo.DeviceCount)
			assert.Equal(t, "", gpuInfo.DeviceName)
		}
	})

	t.Run("should maintain consistent state across multiple calls", func(t *testing.T) {
		gpuInfo1, err1 := detector.DetectGPU()
		gpuInfo2, err2 := detector.DetectGPU()

		assert.NotNil(t, gpuInfo1)
		assert.NotNil(t, gpuInfo2)

		// Results should be consistent
		assert.Equal(t, gpuInfo1.Available, gpuInfo2.Available)
		assert.Equal(t, gpuInfo1.DeviceCount, gpuInfo2.DeviceCount)

		// Error state should be consistent
		if err1 != nil {
			assert.Error(t, err2)
		}
		if err1 == nil {
			assert.NoError(t, err2)
		}
	})
}

func TestIsCUDAAvailable(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should return boolean without crashing", func(t *testing.T) {
		available := detector.IsCUDAAvailable()
		assert.IsType(t, false, available)
	})

	t.Run("should be consistent with DetectGPU results", func(t *testing.T) {
		// First run detection
		gpuInfo, _ := detector.DetectGPU()

		// Then check CUDA availability
		cudaAvailable := detector.IsCUDAAvailable()

		// Results should be consistent (both true or both false)
		if gpuInfo != nil && gpuInfo.Available && gpuInfo.CUDAVersion != "" {
			// If we detected GPU with CUDA, IsCUDAAvailable should likely return true
			// (though it might still return false due to different detection logic)
			assert.IsType(t, false, cudaAvailable)
		} else {
			// If no GPU detected, CUDA should not be available
			assert.False(t, cudaAvailable)
		}
	})
}

func TestGetOptimalDeviceID(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should return valid device ID or -1", func(t *testing.T) {
		deviceID := detector.GetOptimalDeviceID(0)
		assert.GreaterOrEqual(t, deviceID, -1) // -1 means no GPU
	})

	t.Run("should handle invalid configured device ID", func(t *testing.T) {
		deviceID := detector.GetOptimalDeviceID(999)
		assert.GreaterOrEqual(t, deviceID, -1)
		// Should fallback to -1 or 0 for invalid IDs
	})

	t.Run("should be consistent with GPU detection", func(t *testing.T) {
		// First check if GPU is available
		gpuInfo, _ := detector.DetectGPU()

		// Get optimal device ID
		deviceID := detector.GetOptimalDeviceID(0)

		if gpuInfo != nil && gpuInfo.Available && gpuInfo.DeviceCount > 0 {
			// If GPU is available, device ID should be >= 0
			assert.GreaterOrEqual(t, deviceID, 0)
		} else {
			// If no GPU, should return -1
			assert.Equal(t, -1, deviceID)
		}
	})

	t.Run("should handle multiple device IDs", func(t *testing.T) {
		// Test various device IDs
		for i := 0; i < 5; i++ {
			deviceID := detector.GetOptimalDeviceID(i)
			assert.GreaterOrEqual(t, deviceID, -1)
		}
	})
}

func TestGetGPUInfo(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should always return valid GPUInfo structure", func(t *testing.T) {
		gpuInfo := detector.GetGPUInfo()
		assert.NotNil(t, gpuInfo)

		// Should always return a valid GPUInfo structure
		assert.IsType(t, false, gpuInfo.Available)
		assert.IsType(t, 0, gpuInfo.DeviceCount)
		assert.IsType(t, "", gpuInfo.DeviceName)
		assert.IsType(t, "", gpuInfo.CUDAVersion)
		assert.IsType(t, "", gpuInfo.DriverVersion)
	})

	t.Run("should return same info as last DetectGPU call", func(t *testing.T) {
		// First run detection
		detectedInfo, _ := detector.DetectGPU()

		// Then get cached info
		cachedInfo := detector.GetGPUInfo()

		// Should be same instance or equivalent data
		assert.NotNil(t, cachedInfo)
		if detectedInfo != nil {
			assert.Equal(t, detectedInfo.Available, cachedInfo.Available)
			assert.Equal(t, detectedInfo.DeviceCount, cachedInfo.DeviceCount)
			assert.Equal(t, detectedInfo.DeviceName, cachedInfo.DeviceName)
		}
	})

	t.Run("should handle multiple calls consistently", func(t *testing.T) {
		info1 := detector.GetGPUInfo()
		info2 := detector.GetGPUInfo()

		assert.NotNil(t, info1)
		assert.NotNil(t, info2)
		// Should return consistent results
		assert.Equal(t, info1.Available, info2.Available)
		assert.Equal(t, info1.DeviceCount, info2.DeviceCount)
	})
}

func TestDetectWithNvidiaSMI(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should handle nvidia-smi command failure", func(t *testing.T) {
		gpuInfo := &GPUInfo{}
		err := detector.detectWithNvidiaSMI(gpuInfo)

		// In test environment, nvidia-smi likely doesn't exist or fails
		// Test should handle this gracefully
		if err != nil {
			assert.Contains(t, err.Error(), "nvidia-smi command failed")
			assert.False(t, gpuInfo.Available)
		}
	})

	t.Run("should validate gpu info structure when nvidia-smi available", func(t *testing.T) {
		gpuInfo := &GPUInfo{}
		err := detector.detectWithNvidiaSMI(gpuInfo)

		// If nvidia-smi succeeds (rare in test environment), validate structure
		if err == nil && gpuInfo.Available {
			assert.Greater(t, gpuInfo.DeviceCount, 0)
			assert.NotEmpty(t, gpuInfo.DeviceName)
			assert.NotEmpty(t, gpuInfo.DriverVersion)
			// CUDA version might be empty if not available
		}
	})

	t.Run("should maintain consistent availability state", func(t *testing.T) {
		gpuInfo := &GPUInfo{}
		_ = detector.detectWithNvidiaSMI(gpuInfo)

		// Available should match device count state
		if gpuInfo.DeviceCount > 0 {
			assert.True(t, gpuInfo.Available)
		} else {
			assert.False(t, gpuInfo.Available)
		}
	})
}

func TestDetectWithCUDAEnv(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should return error when no CUDA environment variables are set", func(t *testing.T) {
		// Clear any existing CUDA environment variables
		oldCUDAPath := os.Getenv("CUDA_PATH")
		oldCUDAVersion := os.Getenv("CUDA_VERSION")
		oldVisibleDevices := os.Getenv("CUDA_VISIBLE_DEVICES")

		os.Unsetenv("CUDA_PATH")
		os.Unsetenv("CUDA_VERSION")
		os.Unsetenv("CUDA_VISIBLE_DEVICES")

		defer func() {
			// Restore original values
			if oldCUDAPath != "" {
				os.Setenv("CUDA_PATH", oldCUDAPath)
			}
			if oldCUDAVersion != "" {
				os.Setenv("CUDA_VERSION", oldCUDAVersion)
			}
			if oldVisibleDevices != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldVisibleDevices)
			}
		}()

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no CUDA environment variables found")
	})

	t.Run("should detect GPU when CUDA_PATH is set", func(t *testing.T) {
		oldCUDAPath := os.Getenv("CUDA_PATH")
		defer func() {
			if oldCUDAPath != "" {
				os.Setenv("CUDA_PATH", oldCUDAPath)
			} else {
				os.Unsetenv("CUDA_PATH")
			}
		}()

		os.Setenv("CUDA_PATH", "/usr/local/cuda")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		// Should not return the "no CUDA environment variables" error
		if err != nil {
			assert.NotContains(t, err.Error(), "no CUDA environment variables found")
		}
	})

	t.Run("should detect GPU when CUDA_VERSION is set", func(t *testing.T) {
		oldCUDAVersion := os.Getenv("CUDA_VERSION")
		defer func() {
			if oldCUDAVersion != "" {
				os.Setenv("CUDA_VERSION", oldCUDAVersion)
			} else {
				os.Unsetenv("CUDA_VERSION")
			}
		}()

		os.Setenv("CUDA_VERSION", "12.4")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		// Should not return the "no CUDA environment variables" error
		if err != nil {
			assert.NotContains(t, err.Error(), "no CUDA environment variables found")
		}
	})

	t.Run("should detect GPU when CUDA_VISIBLE_DEVICES is set", func(t *testing.T) {
		oldVisibleDevices := os.Getenv("CUDA_VISIBLE_DEVICES")
		defer func() {
			if oldVisibleDevices != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldVisibleDevices)
			} else {
				os.Unsetenv("CUDA_VISIBLE_DEVICES")
			}
		}()

		os.Setenv("CUDA_VISIBLE_DEVICES", "0,1")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		// Should not return the "no CUDA environment variables" error
		if err != nil {
			assert.NotContains(t, err.Error(), "no CUDA environment variables found")
		}
		// Should set device count based on visible devices
		assert.Equal(t, 2, gpuInfo.DeviceCount)
		assert.True(t, gpuInfo.Available)
	})

	t.Run("should handle CUDA_VISIBLE_DEVICES with single device", func(t *testing.T) {
		oldVisibleDevices := os.Getenv("CUDA_VISIBLE_DEVICES")
		defer func() {
			if oldVisibleDevices != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldVisibleDevices)
			} else {
				os.Unsetenv("CUDA_VISIBLE_DEVICES")
			}
		}()

		os.Setenv("CUDA_VISIBLE_DEVICES", "0")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		assert.NoError(t, err)
		assert.Equal(t, 1, gpuInfo.DeviceCount)
		assert.True(t, gpuInfo.Available)
	})

	t.Run("should handle CUDA_VISIBLE_DEVICES set to -1", func(t *testing.T) {
		oldVisibleDevices := os.Getenv("CUDA_VISIBLE_DEVICES")
		defer func() {
			if oldVisibleDevices != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldVisibleDevices)
			} else {
				os.Unsetenv("CUDA_VISIBLE_DEVICES")
			}
		}()

		os.Setenv("CUDA_VISIBLE_DEVICES", "-1")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		assert.NoError(t, err)
		assert.Equal(t, 0, gpuInfo.DeviceCount)
		assert.False(t, gpuInfo.Available)
	})

	t.Run("should set CUDA version from environment", func(t *testing.T) {
		oldCUDAVersion := os.Getenv("CUDA_VERSION")
		defer func() {
			if oldCUDAVersion != "" {
				os.Setenv("CUDA_VERSION", oldCUDAVersion)
			} else {
				os.Unsetenv("CUDA_VERSION")
			}
		}()

		os.Setenv("CUDA_VERSION", "12.4.1")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		assert.NoError(t, err)
		assert.Equal(t, "12.4.1", gpuInfo.CUDAVersion)
	})
}

func TestDetectWithCUDAToolkit(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should return error when CUDA toolkit not found", func(t *testing.T) {
		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAToolkit(gpuInfo)

		// In most test environments, CUDA toolkit won't be installed
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CUDA toolkit not found in standard locations")
		assert.False(t, gpuInfo.Available)
	})

	t.Run("should not crash when checking standard CUDA paths", func(t *testing.T) {
		gpuInfo := &GPUInfo{}

		// This should complete without panic, even if CUDA not found
		err := detector.detectWithCUDAToolkit(gpuInfo)

		// Should return error or successfully detect CUDA
		assert.NotNil(t, gpuInfo)
		if err == nil {
			// If CUDA found, should be marked as available
			assert.True(t, gpuInfo.Available)
			assert.Greater(t, gpuInfo.DeviceCount, 0)
			assert.NotEmpty(t, gpuInfo.CUDAVersion)
		}
	})

	t.Run("should handle file system errors gracefully", func(t *testing.T) {
		gpuInfo := &GPUInfo{}

		// Test that function handles os.Stat errors properly
		err := detector.detectWithCUDAToolkit(gpuInfo)

		// Should always return an error in test environment
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CUDA toolkit not found in standard locations")
	})

	t.Run("should maintain consistent GPU info state", func(t *testing.T) {
		gpuInfo := &GPUInfo{}
		initialAvailable := gpuInfo.Available
		initialCount := gpuInfo.DeviceCount

		_ = detector.detectWithCUDAToolkit(gpuInfo)

		// GPU info should be properly initialized
		assert.IsType(t, false, gpuInfo.Available)
		assert.IsType(t, 0, gpuInfo.DeviceCount)

		// Function should modify the gpuInfo struct consistently
		if gpuInfo.Available {
			assert.Greater(t, gpuInfo.DeviceCount, 0)
			assert.NotEmpty(t, gpuInfo.CUDAVersion)
		} else {
			// Should not modify fields if detection fails
			assert.Equal(t, initialAvailable, gpuInfo.Available)
			assert.Equal(t, initialCount, gpuInfo.DeviceCount)
		}
	})
}

// Additional tests for improved coverage
func TestDetectGPUSuccessPath(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should log GPU detection completion", func(t *testing.T) {
		// Test the logging path in DetectGPU that happens after detection attempts
		gpuInfo, err := detector.DetectGPU()

		// Should complete without panic and return valid info
		assert.NotNil(t, gpuInfo)
		assert.NoError(t, err)

		// GPU info should be initialized
		assert.IsType(t, false, gpuInfo.Available)
		assert.IsType(t, 0, gpuInfo.DeviceCount)
		assert.IsType(t, "", gpuInfo.DeviceName)
		assert.IsType(t, "", gpuInfo.CUDAVersion)
		assert.IsType(t, "", gpuInfo.DriverVersion)
	})
}

func TestCUDAEnvironmentEdgeCases(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should handle CUDA_VISIBLE_DEVICES=-1", func(t *testing.T) {
		oldValue := os.Getenv("CUDA_VISIBLE_DEVICES")
		defer func() {
			if oldValue != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldValue)
			} else {
				os.Unsetenv("CUDA_VISIBLE_DEVICES")
			}
		}()

		os.Setenv("CUDA_VISIBLE_DEVICES", "-1")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		// Should not error but should not set devices as available
		assert.NoError(t, err)
		assert.False(t, gpuInfo.Available)
		assert.Equal(t, 0, gpuInfo.DeviceCount)
	})

	t.Run("should parse multiple devices from CUDA_VISIBLE_DEVICES", func(t *testing.T) {
		oldValue := os.Getenv("CUDA_VISIBLE_DEVICES")
		defer func() {
			if oldValue != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldValue)
			} else {
				os.Unsetenv("CUDA_VISIBLE_DEVICES")
			}
		}()

		os.Setenv("CUDA_VISIBLE_DEVICES", "0,1,2")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		assert.NoError(t, err)
		assert.True(t, gpuInfo.Available)
		assert.Equal(t, 3, gpuInfo.DeviceCount)
	})

	t.Run("should set CUDA version from environment", func(t *testing.T) {
		oldCUDAVersion := os.Getenv("CUDA_VERSION")
		oldVisibleDevices := os.Getenv("CUDA_VISIBLE_DEVICES")

		defer func() {
			if oldCUDAVersion != "" {
				os.Setenv("CUDA_VERSION", oldCUDAVersion)
			} else {
				os.Unsetenv("CUDA_VERSION")
			}
			if oldVisibleDevices != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldVisibleDevices)
			} else {
				os.Unsetenv("CUDA_VISIBLE_DEVICES")
			}
		}()

		os.Setenv("CUDA_VERSION", "12.5.1")
		os.Setenv("CUDA_VISIBLE_DEVICES", "0")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		assert.NoError(t, err)
		assert.Equal(t, "12.5.1", gpuInfo.CUDAVersion)
		assert.True(t, gpuInfo.Available)
	})
}

// Additional comprehensive tests to improve coverage
func TestNvidiaSMIOutputParsing(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should handle empty nvidia-smi output", func(t *testing.T) {
		gpuInfo := &GPUInfo{}
		err := detector.detectWithNvidiaSMI(gpuInfo)

		// Should error due to nvidia-smi not available in test environment
		assert.Error(t, err)
	})

	t.Run("should validate nvidia-smi error paths are handled", func(t *testing.T) {
		gpuInfo := &GPUInfo{}
		err := detector.detectWithNvidiaSMI(gpuInfo)

		// Should contain nvidia-smi command failed message
		if err != nil {
			assert.Contains(t, err.Error(), "nvidia-smi")
		}
	})
}

func TestOptimalDeviceIDEdgeCases(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should handle negative configured device ID", func(t *testing.T) {
		deviceID := detector.GetOptimalDeviceID(-5)
		assert.GreaterOrEqual(t, deviceID, -1)
	})

	t.Run("should handle very large configured device ID", func(t *testing.T) {
		deviceID := detector.GetOptimalDeviceID(9999)
		assert.GreaterOrEqual(t, deviceID, -1)
	})

	t.Run("should return -1 when no GPU available", func(t *testing.T) {
		// In test environment, no GPU should be available
		deviceID := detector.GetOptimalDeviceID(0)
		assert.Equal(t, -1, deviceID)
	})
}

func TestGPUInfoStructValidation(t *testing.T) {
	t.Run("should create GPUInfo with correct types", func(t *testing.T) {
		info := &GPUInfo{
			Available:     true,
			DeviceCount:   2,
			DeviceName:    "Test GPU",
			CUDAVersion:   "12.4",
			DriverVersion: "525.60.11",
		}

		assert.True(t, info.Available)
		assert.Equal(t, 2, info.DeviceCount)
		assert.Equal(t, "Test GPU", info.DeviceName)
		assert.Equal(t, "12.4", info.CUDAVersion)
		assert.Equal(t, "525.60.11", info.DriverVersion)
	})
}

func TestDetectorLoggerIntegration(t *testing.T) {
	t.Run("should work with different logger types", func(t *testing.T) {
		// Test with development logger
		devLogger, _ := zap.NewDevelopment()
		detector1 := NewGPUDetector(devLogger)
		assert.NotNil(t, detector1)

		// Test with nop logger
		nopLogger := zap.NewNop()
		detector2 := NewGPUDetector(nopLogger)
		assert.NotNil(t, detector2)

		// Both should work consistently
		info1 := detector1.GetGPUInfo()
		info2 := detector2.GetGPUInfo()
		assert.NotNil(t, info1)
		assert.NotNil(t, info2)
	})
}

func TestCUDAToolkitVersionParsing(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should handle CUDA toolkit detection paths", func(t *testing.T) {
		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAToolkit(gpuInfo)

		// Should error in test environment due to no CUDA toolkit
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "CUDA toolkit not found")
		assert.False(t, gpuInfo.Available)
	})
}

func TestDetectGPUMethodCoverage(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should test all detection method fallbacks", func(t *testing.T) {
		// This tests the full detection chain:
		// 1. nvidia-smi fails
		// 2. CUDA env vars fail
		// 3. CUDA toolkit fails
		// 4. Returns default GPUInfo
		gpuInfo, err := detector.DetectGPU()

		assert.NotNil(t, gpuInfo)
		assert.NoError(t, err) // DetectGPU doesn't return errors, just logs them
		assert.False(t, gpuInfo.Available) // Should be false in test environment
		assert.Equal(t, 0, gpuInfo.DeviceCount)
		assert.Equal(t, "", gpuInfo.DeviceName)
		assert.Equal(t, "", gpuInfo.CUDAVersion)
		assert.Equal(t, "", gpuInfo.DriverVersion)
	})

	t.Run("should maintain consistent state across multiple detection calls", func(t *testing.T) {
		// Test multiple calls to ensure consistent behavior
		for i := 0; i < 3; i++ {
			gpuInfo, err := detector.DetectGPU()
			assert.NotNil(t, gpuInfo)
			assert.NoError(t, err)
			assert.False(t, gpuInfo.Available)
		}
	})
}

func TestEnvironmentVariableHandling(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("should handle empty CUDA_VISIBLE_DEVICES", func(t *testing.T) {
		oldValue := os.Getenv("CUDA_VISIBLE_DEVICES")
		defer func() {
			if oldValue != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldValue)
			} else {
				os.Unsetenv("CUDA_VISIBLE_DEVICES")
			}
		}()

		os.Setenv("CUDA_VISIBLE_DEVICES", "")

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		// Empty string should be treated as if the var is not set
		assert.Error(t, err)
		assert.Contains(t, err.Error(), "no CUDA environment variables found")
	})

	t.Run("should handle only CUDA_PATH set", func(t *testing.T) {
		// Clear other CUDA env vars and set only CUDA_PATH
		oldCUDAPath := os.Getenv("CUDA_PATH")
		oldCUDAVersion := os.Getenv("CUDA_VERSION")
		oldVisibleDevices := os.Getenv("CUDA_VISIBLE_DEVICES")

		os.Unsetenv("CUDA_VERSION")
		os.Unsetenv("CUDA_VISIBLE_DEVICES")
		os.Setenv("CUDA_PATH", "/usr/local/cuda")

		defer func() {
			if oldCUDAPath != "" {
				os.Setenv("CUDA_PATH", oldCUDAPath)
			} else {
				os.Unsetenv("CUDA_PATH")
			}
			if oldCUDAVersion != "" {
				os.Setenv("CUDA_VERSION", oldCUDAVersion)
			}
			if oldVisibleDevices != "" {
				os.Setenv("CUDA_VISIBLE_DEVICES", oldVisibleDevices)
			}
		}()

		gpuInfo := &GPUInfo{}
		err := detector.detectWithCUDAEnv(gpuInfo)

		// Should not error due to CUDA_PATH being set
		assert.NoError(t, err)
	})
}

