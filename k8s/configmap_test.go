package k8s

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// TestConfigMapManifest validates the Kubernetes ConfigMap configuration
func TestConfigMapManifest(t *testing.T) {
	// Test case: ConfigMap should have correct configuration
	t.Run("ConfigMap has correct configuration", func(t *testing.T) {
		// ARRANGE: Expected ConfigMap configuration
		expectedName := "radiocontestwinner-config"
		expectedStreamURL := "https://ais-sa1.streamon.fm:443/7346_48k.aac"
		expectedModelPath := "./models/ggml-base.en.bin"
		expectedGPUMemoryLimit := ""

		// ACT: Read and parse the ConfigMap manifest
		configMap, err := loadConfigMapManifest()

		// ASSERT: Validate ConfigMap configuration
		assert.NoError(t, err, "Should load ConfigMap manifest without errors")
		assert.NotNil(t, configMap, "ConfigMap should not be nil")

		// Validate ConfigMap metadata
		assert.Equal(t, expectedName, configMap.Metadata.Name, "ConfigMap name should match")
		assert.Contains(t, configMap.Metadata.Labels, "app", "Should have app label")
		assert.Equal(t, "radiocontestwinner", configMap.Metadata.Labels["app"], "App label should match")

		// Validate configuration data
		data := configMap.Data
		assert.NotNil(t, data, "Should have configuration data")
		assert.Contains(t, data, "config.yaml", "Should have config.yaml entry")

		// Parse the embedded configuration
		configContent := data["config.yaml"]
		assert.Contains(t, configContent, "stream:", "Should have stream configuration")
		assert.Contains(t, configContent, "whisper:", "Should have whisper configuration")
		assert.Contains(t, configContent, "buffer:", "Should have buffer configuration")
		assert.Contains(t, configContent, "allowlist:", "Should have allowlist configuration")
		assert.Contains(t, configContent, "gpu:", "Should have GPU configuration")

		// Parse the embedded YAML to validate specific values
		var config map[string]interface{}
		if err := yaml.Unmarshal([]byte(configContent), &config); err == nil {
			// Validate stream configuration
			if stream, ok := config["stream"].(map[interface{}]interface{}); ok {
				if url, ok := stream["url"].(string); ok {
					assert.Equal(t, expectedStreamURL, url, "Stream URL should match")
				}
			}

			// Validate whisper configuration
			if whisper, ok := config["whisper"].(map[interface{}]interface{}); ok {
				if modelPath, ok := whisper["model_path"].(string); ok {
					assert.Equal(t, expectedModelPath, modelPath, "Model path should match")
				}
			}

			// Validate buffer configuration
			if buffer, ok := config["buffer"].(map[interface{}]interface{}); ok {
				if duration, ok := buffer["duration_ms"].(int); ok {
					assert.Equal(t, 2500, duration, "Buffer duration should be 2500ms")
				}
			}

			// Validate GPU configuration
			if gpu, ok := config["gpu"].(map[interface{}]interface{}); ok {
				if enabled, ok := gpu["enabled"].(bool); ok {
					assert.True(t, enabled, "GPU should be enabled")
				}
				if autoDetect, ok := gpu["auto_detect"].(bool); ok {
					assert.True(t, autoDetect, "GPU auto-detect should be enabled")
				}
				if deviceID, ok := gpu["device_id"].(int); ok {
					assert.Equal(t, 0, deviceID, "GPU device ID should be 0")
				}
				if memoryLimit, ok := gpu["memory_limit"].(string); ok {
					assert.Equal(t, expectedGPUMemoryLimit, memoryLimit, "GPU memory limit should be empty")
				}
			}
		}
	})
}

// TestConfigMapValidation validates ConfigMap configuration validation
func TestConfigMapValidation(t *testing.T) {
	t.Run("ConfigMap configuration is valid", func(t *testing.T) {
		// ACT: Read ConfigMap manifest
		configMap, err := loadConfigMapManifest()

		// ASSERT: Validate configuration completeness
		assert.NoError(t, err, "Should load ConfigMap manifest without errors")
		assert.NotNil(t, configMap, "ConfigMap should not be nil")

		data := configMap.Data
		assert.NotNil(t, data, "Should have configuration data")

		configContent := data["config.yaml"]
		assert.NotEmpty(t, configContent, "Config content should not be empty")

		// Validate required configuration sections are present
		requiredSections := []string{
			"stream:", "whisper:", "buffer:", "allowlist:", "debug_mode:", "gpu:", "performance:",
		}

		for _, section := range requiredSections {
			assert.Contains(t, configContent, section, "Should have required section: %s", section)
		}

		// Validate specific configuration values
		assert.Contains(t, configContent, "url: \"https://ais-sa1.streamon.fm:443/7346_48k.aac\"", "Should have correct stream URL")
		assert.Contains(t, configContent, "model_path: \"./models/ggml-base.en.bin\"", "Should have correct model path")
		assert.Contains(t, configContent, "duration_ms: 2500", "Should have correct buffer duration")
		assert.Contains(t, configContent, "enabled: true", "GPU should be enabled")
		assert.Contains(t, configContent, "auto_detect: true", "GPU auto-detect should be enabled")
	})
}

// TestConfigMapLabels validates ConfigMap labels and metadata
func TestConfigMapLabels(t *testing.T) {
	t.Run("ConfigMap has correct labels and metadata", func(t *testing.T) {
		// ARRANGE: Expected labels
		expectedLabels := map[string]string{
			"app":       "radiocontestwinner",
			"version":   "v3.4",
			"component": "configuration",
		}

		// ACT: Read ConfigMap manifest
		configMap, err := loadConfigMapManifest()

		// ASSERT: Validate labels
		assert.NoError(t, err, "Should load ConfigMap manifest without errors")
		assert.NotNil(t, configMap, "ConfigMap should not be nil")

		labels := configMap.Metadata.Labels
		assert.NotNil(t, labels, "Should have labels")

		for key, expectedValue := range expectedLabels {
			assert.Contains(t, labels, key, "Should have label %s", key)
			assert.Equal(t, expectedValue, labels[key], "Label %s should have correct value", key)
		}
	})
}

// loadConfigMapManifest is a helper function to load the ConfigMap manifest
func loadConfigMapManifest() (*ConfigMap, error) {
	// Read the configmap.yaml file
	data, err := os.ReadFile("configmap.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read configmap.yaml: %w", err)
	}

	// Parse the YAML
	var configMap ConfigMap
	if err := yaml.Unmarshal(data, &configMap); err != nil {
		return nil, fmt.Errorf("failed to parse configmap.yaml: %w", err)
	}

	return &configMap, nil
}

// ConfigMap represents the Kubernetes ConfigMap structure
type ConfigMap struct {
	Metadata ObjectMeta            `yaml:"metadata" json:"metadata"`
	Data     map[string]string      `yaml:"data" json:"data"`
}