package k8s

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// TestServiceManifest validates the Kubernetes Service manifest configuration
func TestServiceManifest(t *testing.T) {
	// Test case: Service manifest should have correct configuration
	t.Run("Service manifest has correct configuration", func(t *testing.T) {
		// ARRANGE: Expected service configuration
		expectedServiceName := "radiocontestwinner"
		expectedServiceType := "ClusterIP"
		expectedPort := int32(8080)
		expectedTargetPort := "http"
		expectedPortName := "http"
		expectedProtocol := "TCP"

		// ACT: Read and parse the service manifest
		service, err := loadServiceManifest()

		// ASSERT: Validate service configuration
		assert.NoError(t, err, "Should load service manifest without errors")
		assert.NotNil(t, service, "Service should not be nil")

		// Validate service metadata
		assert.Equal(t, expectedServiceName, service.Metadata.Name, "Service name should match")
		assert.Contains(t, service.Metadata.Labels, "app", "Should have app label")
		assert.Equal(t, expectedServiceName, service.Metadata.Labels["app"], "App label should match")

		// Validate service spec
		assert.Equal(t, expectedServiceType, service.Spec.Type, "Service type should be ClusterIP")
		assert.NotNil(t, service.Spec.Selector, "Should have selector")
		assert.Equal(t, expectedServiceName, service.Spec.Selector["app"], "Selector should match app label")

		// Validate ports
		assert.Len(t, service.Spec.Ports, 1, "Should have exactly one port")
		port := service.Spec.Ports[0]
		assert.Equal(t, expectedPort, port.Port, "Port should be 8080")
		assert.Equal(t, expectedTargetPort, port.TargetPort, "Target port should be http")
		assert.Equal(t, expectedPortName, port.Name, "Port name should be http")
		assert.Equal(t, expectedProtocol, port.Protocol, "Protocol should be TCP")
	})
}

// TestServiceAnnotations validates service annotations
func TestServiceAnnotations(t *testing.T) {
	t.Run("Service has correct annotations", func(t *testing.T) {
		// ARRANGE: Expected annotations
		expectedAnnotations := map[string]string{
			"prometheus.io/scrape": "true",
			"prometheus.io/port":   "8080",
			"prometheus.io/path":   "/metrics",
		}

		// ACT: Read service manifest
		service, err := loadServiceManifest()

		// ASSERT: Validate annotations
		assert.NoError(t, err, "Should load service manifest without errors")
		assert.NotNil(t, service, "Service should not be nil")

		annotations := service.Metadata.Annotations
		assert.NotNil(t, annotations, "Should have annotations")

		for key, expectedValue := range expectedAnnotations {
			assert.Contains(t, annotations, key, "Should have annotation %s", key)
			assert.Equal(t, expectedValue, annotations[key], "Annotation %s should have correct value", key)
		}
	})
}

// TestServiceSelector validates service selector configuration
func TestServiceSelector(t *testing.T) {
	t.Run("Service has correct selector configuration", func(t *testing.T) {
		// ARRANGE: Expected selector labels
		expectedSelector := map[string]string{
			"app":       "radiocontestwinner",
			"component": "audio-transcription",
		}

		// ACT: Read service manifest
		service, err := loadServiceManifest()

		// ASSERT: Validate selector
		assert.NoError(t, err, "Should load service manifest without errors")
		assert.NotNil(t, service, "Service should not be nil")

		selector := service.Spec.Selector
		assert.NotNil(t, selector, "Should have selector")

		for key, expectedValue := range expectedSelector {
			assert.Contains(t, selector, key, "Should have selector label %s", key)
			assert.Equal(t, expectedValue, selector[key], "Selector %s should have correct value", key)
		}
	})
}

// loadServiceManifest is a helper function to load the service manifest
func loadServiceManifest() (*Service, error) {
	// Read the service.yaml file
	data, err := os.ReadFile("service.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read service.yaml: %w", err)
	}

	// Parse the YAML
	var service Service
	if err := yaml.Unmarshal(data, &service); err != nil {
		return nil, fmt.Errorf("failed to parse service.yaml: %w", err)
	}

	return &service, nil
}

// Service represents the Kubernetes Service structure
type Service struct {
	Metadata ObjectMeta     `yaml:"metadata" json:"metadata"`
	Spec     ServiceSpec     `yaml:"spec" json:"spec"`
}

// ServiceSpec represents the Kubernetes Service specification
type ServiceSpec struct {
	Type     string            `yaml:"type" json:"type"`
	Selector map[string]string `yaml:"selector" json:"selector"`
	Ports    []ServicePort     `yaml:"ports" json:"ports"`
}

// ServicePort represents a service port
type ServicePort struct {
	Port       int32  `yaml:"port" json:"port"`
	TargetPort string `yaml:"targetPort" json:"targetPort"`
	Name       string `yaml:"name" json:"name"`
	Protocol   string `yaml:"protocol" json:"protocol"`
}

