package k8s

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// TestPVCModels validates the models PersistentVolumeClaim configuration
func TestPVCModels(t *testing.T) {
	// Test case: Models PVC should have correct configuration
	t.Run("Models PVC has correct configuration", func(t *testing.T) {
		// ARRANGE: Expected models PVC configuration
		expectedName := "radiocontestwinner-models"
		expectedStorageClass := "standard"
		expectedAccessMode := "ReadWriteOnce"
		expectedSize := "5Gi"

		// ACT: Read and parse the PVC manifest
		pvc, err := loadPVCManifest("pvc-models.yaml")

		// ASSERT: Validate PVC configuration
		assert.NoError(t, err, "Should load models PVC manifest without errors")
		assert.NotNil(t, pvc, "PVC should not be nil")

		// Validate PVC metadata
		assert.Equal(t, expectedName, pvc.Metadata.Name, "PVC name should match")
		assert.Contains(t, pvc.Metadata.Labels, "app", "Should have app label")
		assert.Equal(t, "radiocontestwinner", pvc.Metadata.Labels["app"], "App label should match")
		assert.Equal(t, "models", pvc.Metadata.Labels["component"], "Component label should be models")

		// Validate PVC spec
		assert.Equal(t, expectedAccessMode, pvc.Spec.AccessModes[0], "Access mode should be ReadWriteOnce")
		assert.Equal(t, expectedSize, pvc.Spec.Resources.Requests.Storage, "Storage size should be 5Gi")

		// Validate storage class if specified
		if pvc.Spec.StorageClassName != nil {
			assert.Equal(t, expectedStorageClass, *pvc.Spec.StorageClassName, "Storage class should be standard")
		}
	})
}

// TestPVCOutput validates the output PersistentVolumeClaim configuration
func TestPVCOutput(t *testing.T) {
	// Test case: Output PVC should have correct configuration
	t.Run("Output PVC has correct configuration", func(t *testing.T) {
		// ARRANGE: Expected output PVC configuration
		expectedName := "radiocontestwinner-output"
		expectedStorageClass := "standard"
		expectedAccessMode := "ReadWriteOnce"
		expectedSize := "10Gi" // Larger size for output files

		// ACT: Read and parse the PVC manifest
		pvc, err := loadPVCManifest("pvc-output.yaml")

		// ASSERT: Validate PVC configuration
		assert.NoError(t, err, "Should load output PVC manifest without errors")
		assert.NotNil(t, pvc, "PVC should not be nil")

		// Validate PVC metadata
		assert.Equal(t, expectedName, pvc.Metadata.Name, "PVC name should match")
		assert.Contains(t, pvc.Metadata.Labels, "app", "Should have app label")
		assert.Equal(t, "radiocontestwinner", pvc.Metadata.Labels["app"], "App label should match")
		assert.Equal(t, "output", pvc.Metadata.Labels["component"], "Component label should be output")

		// Validate PVC spec
		assert.Equal(t, expectedAccessMode, pvc.Spec.AccessModes[0], "Access mode should be ReadWriteOnce")
		assert.Equal(t, expectedSize, pvc.Spec.Resources.Requests.Storage, "Storage size should be 10Gi")

		// Validate storage class if specified
		if pvc.Spec.StorageClassName != nil {
			assert.Equal(t, expectedStorageClass, *pvc.Spec.StorageClassName, "Storage class should be standard")
		}
	})
}

// TestPVCAnnotations validates PVC annotations and metadata
func TestPVCAnnotations(t *testing.T) {
	t.Run("PVC has correct annotations", func(t *testing.T) {
		// ARRANGE: Expected annotations for models PVC
		expectedAnnotations := map[string]string{
			"description": "Persistent storage for Whisper AI models and audio processing files",
			"backup": "true",
			"retention": "long-term",
		}

		// ACT: Read models PVC manifest
		pvc, err := loadPVCManifest("pvc-models.yaml")

		// ASSERT: Validate annotations
		assert.NoError(t, err, "Should load models PVC manifest without errors")
		assert.NotNil(t, pvc, "PVC should not be nil")

		annotations := pvc.Metadata.Annotations
		assert.NotNil(t, annotations, "Should have annotations")

		for key, expectedValue := range expectedAnnotations {
			assert.Contains(t, annotations, key, "Should have annotation %s", key)
			assert.Equal(t, expectedValue, annotations[key], "Annotation %s should have correct value", key)
		}
	})
}

// TestPVCStorageConfiguration validates storage configuration
func TestPVCStorageConfiguration(t *testing.T) {
	t.Run("PVC storage configuration is appropriate", func(t *testing.T) {
		// ACT: Read both PVC manifests
		modelsPVC, err := loadPVCManifest("pvc-models.yaml")
		assert.NoError(t, err, "Should load models PVC manifest without errors")
		assert.NotNil(t, modelsPVC, "Models PVC should not be nil")

		outputPVC, err := loadPVCManifest("pvc-output.yaml")
		assert.NoError(t, err, "Should load output PVC manifest without errors")
		assert.NotNil(t, outputPVC, "Output PVC should not be nil")

		// ASSERT: Validate storage sizing is appropriate
		modelsStorage := modelsPVC.Spec.Resources.Requests.Storage
		outputStorage := outputPVC.Spec.Resources.Requests.Storage

		// Models should be smaller than output (models: ~1-2GB for Whisper models)
		assert.Equal(t, "5Gi", modelsStorage, "Models PVC should be 5Gi for Whisper models")

		// Output should be larger for contest results and logs
		assert.Equal(t, "10Gi", outputStorage, "Output PVC should be 10Gi for contest results")

		// Both should use appropriate access modes
		assert.Equal(t, "ReadWriteOnce", modelsPVC.Spec.AccessModes[0], "Models should use ReadWriteOnce")
		assert.Equal(t, "ReadWriteOnce", outputPVC.Spec.AccessModes[0], "Output should use ReadWriteOnce")

		// Both should have volume mode filesystem
		assert.Equal(t, "Filesystem", modelsPVC.Spec.VolumeMode, "Models should use filesystem volume mode")
		assert.Equal(t, "Filesystem", outputPVC.Spec.VolumeMode, "Output should use filesystem volume mode")
	})
}

// loadPVCManifest is a helper function to load a PVC manifest
func loadPVCManifest(filename string) (*PersistentVolumeClaim, error) {
	// Read the PVC file
	data, err := os.ReadFile(filename)
	if err != nil {
		return nil, fmt.Errorf("failed to read %s: %w", filename, err)
	}

	// Parse the YAML
	var pvc PersistentVolumeClaim
	if err := yaml.Unmarshal(data, &pvc); err != nil {
		return nil, fmt.Errorf("failed to parse %s: %w", filename, err)
	}

	return &pvc, nil
}

// PersistentVolumeClaim represents the Kubernetes PVC structure
type PersistentVolumeClaim struct {
	Metadata ObjectMeta                `yaml:"metadata" json:"metadata"`
	Spec     PVCSpec                   `yaml:"spec" json:"spec"`
}

// PVCSpec represents the PVC specification
type PVCSpec struct {
	AccessModes      []string             `yaml:"accessModes" json:"accessModes"`
	Resources        PVCResourceRequirements  `yaml:"resources" json:"resources"`
	StorageClassName *string              `yaml:"storageClassName" json:"storageClassName"`
	VolumeMode       string               `yaml:"volumeMode" json:"volumeMode"`
}

// PVCResourceRequirements represents PVC resource requirements
type PVCResourceRequirements struct {
	Requests PVCResourceList `yaml:"requests" json:"requests"`
}

// PVCResourceList represents PVC resource limits/requests
type PVCResourceList struct {
	Storage string `yaml:"storage" json:"storage"`
}