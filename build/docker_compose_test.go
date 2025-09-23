package build

import (
	"os"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

func TestDockerComposeGPUConfiguration(t *testing.T) {
	// Read docker-compose.yaml file
	composeFile, err := os.ReadFile("../docker-compose.yaml")
	assert.NoError(t, err)

	var compose struct {
		Services map[string]struct {
			Environment []string `yaml:"environment"`
			Deploy      struct {
				Resources struct {
					Limits struct {
						Memory string `yaml:"memory"`
					} `yaml:"limits"`
					Reservations struct {
						Memory string `yaml:"memory"`
					} `yaml:"reservations"`
				} `yaml:"resources"`
			} `yaml:"deploy"`
			Runtime     string `yaml:"runtime"`
			Healthcheck struct {
				Test     []string `yaml:"test"`
				Interval string   `yaml:"interval"`
				Timeout  string   `yaml:"timeout"`
				Retries  int      `yaml:"retries"`
			} `yaml:"healthcheck"`
		} `yaml:"services"`
	}

	err = yaml.Unmarshal(composeFile, &compose)
	assert.NoError(t, err)

	service, exists := compose.Services["radiocontestwinner"]
	assert.True(t, exists, "radiocontestwinner service should exist")

	// Check for GPU-related environment variables
	envVars := make(map[string]string)
	for _, env := range service.Environment {
		parts := strings.SplitN(env, "=", 2)
		if len(parts) == 2 {
			envVars[parts[0]] = parts[1]
		}
	}

	// Verify GPU environment variables
	assert.Contains(t, envVars, "WHISPER_CUBLAS", "WHISPER_CUBLAS should be configured")
	assert.Contains(t, envVars, "NVIDIA_VISIBLE_DEVICES", "NVIDIA_VISIBLE_DEVICES should be configured")
	assert.Contains(t, envVars, "NVIDIA_DRIVER_CAPABILITIES", "NVIDIA_DRIVER_CAPABILITIES should be configured")

	// Check values
	assert.Equal(t, "true", envVars["WHISPER_CUBLAS"], "WHISPER_CUBLAS should be enabled")
	assert.Equal(t, "all", envVars["NVIDIA_VISIBLE_DEVICES"], "NVIDIA_VISIBLE_DEVICES should be 'all'")
	assert.Equal(t, "compute,utility", envVars["NVIDIA_DRIVER_CAPABILITIES"], "NVIDIA_DRIVER_CAPABILITIES should include compute and utility")

	// Verify runtime configuration
	assert.Equal(t, "nvidia", service.Runtime, "Runtime should be set to nvidia")

	// Verify resource limits for GPU workloads
	assert.Equal(t, "8G", service.Deploy.Resources.Limits.Memory, "Memory limit should be increased for GPU")
	assert.Equal(t, "6G", service.Deploy.Resources.Reservations.Memory, "Memory reservation should be increased for GPU")

	// Verify health check includes GPU check
	found := false
	for _, testItem := range service.Healthcheck.Test {
		if strings.Contains(testItem, "nvidia-smi") {
			found = true
			break
		}
	}
	assert.True(t, found, "Health check should include GPU verification")
}
