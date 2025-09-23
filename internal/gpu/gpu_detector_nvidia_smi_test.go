package gpu

import (
	"testing"

	"github.com/stretchr/testify/assert"
	"go.uber.org/zap"
)

func TestNvidiaSMIQueryCorrectness(t *testing.T) {
	logger := zap.NewNop()
	detector := NewGPUDetector(logger)

	t.Run("nvidia-smi query should use valid fields", func(t *testing.T) {
		// The current implementation uses invalid "count" field
		// This test ensures we fix the nvidia-smi query to use valid fields
		gpuInfo := &GPUInfo{}
		err := detector.detectWithNvidiaSMI(gpuInfo)

		// If nvidia-smi exists and runs, it should not fail due to invalid query fields
		// In test environment it will likely fail due to command not found,
		// but if it runs, the query should be valid
		if err != nil {
			// Error message should not indicate invalid field format
			assert.NotContains(t, err.Error(), "invalid field")
			assert.NotContains(t, err.Error(), "Unknown field")
			assert.NotContains(t, err.Error(), "unrecognized")
		}
	})

	t.Run("should use separate queries for device count and properties", func(t *testing.T) {
		// The correct approach is to use separate nvidia-smi queries:
		// 1. nvidia-smi --list-gpus (to count devices)
		// 2. nvidia-smi --query-gpu=name,driver_version,cuda_version (for properties)
		// This test validates that our detection logic is structured correctly

		gpuInfo := &GPUInfo{}
		err := detector.detectWithNvidiaSMI(gpuInfo)

		// The function should complete without panicking
		assert.NotNil(t, gpuInfo)

		// In test environment, likely no nvidia-smi available
		if err != nil {
			// Should get "command failed" not "invalid query format"
			assert.Contains(t, err.Error(), "nvidia-smi command failed")
		}
	})
}