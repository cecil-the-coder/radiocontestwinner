package k8s

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// TestEndToEndDeployment validates complete Kubernetes deployment configuration
func TestEndToEndDeployment(t *testing.T) {
	// Test case: Complete deployment should be properly configured
	t.Run("Complete deployment configuration is valid", func(t *testing.T) {
		// ARRANGE: Expected deployment configuration
		expectedAppName := "radiocontestwinner"
		expectedConfigMapName := "radiocontestwinner-config"
		expectedSecretName := "radiocontestwinner-secrets"
		expectedServiceName := "radiocontestwinner"

		// ACT: Load all manifests
		deployment, err := loadDeploymentManifest()
		assert.NoError(t, err, "Should load deployment manifest")

		service, err := loadServiceManifest()
		assert.NoError(t, err, "Should load service manifest")

		configMap, err := loadConfigMapManifest()
		assert.NoError(t, err, "Should load configmap manifest")

		secret, err := loadSecretManifest()
		assert.NoError(t, err, "Should load secret manifest")

		modelsPVC, err := loadPVCManifest("pvc-models.yaml")
		assert.NoError(t, err, "Should load models PVC manifest")

		outputPVC, err := loadPVCManifest("pvc-output.yaml")
		assert.NoError(t, err, "Should load output PVC manifest")

		// ASSERT: Validate consistent naming and labeling across all resources
		// Validate deployment configuration
		assert.Equal(t, expectedAppName, deployment.Metadata.Name, "Deployment name should match")
		assert.Contains(t, deployment.Metadata.Labels, "app", "Deployment should have app label")
		assert.Equal(t, expectedAppName, deployment.Metadata.Labels["app"], "Deployment app label should match")

		// Validate service configuration
		assert.Equal(t, expectedServiceName, service.Metadata.Name, "Service name should match")
		assert.Equal(t, expectedAppName, service.Spec.Selector["app"], "Service should select deployment")

		// Validate configmap configuration
		assert.Equal(t, expectedConfigMapName, configMap.Metadata.Name, "ConfigMap name should match")
		assert.Contains(t, configMap.Metadata.Labels, "app", "ConfigMap should have app label")

		// Validate secret configuration
		assert.Equal(t, expectedSecretName, secret.Metadata.Name, "Secret name should match")
		assert.Contains(t, secret.Metadata.Labels, "app", "Secret should have app label")

		// Validate PVC configurations
		assert.Equal(t, "radiocontestwinner-models", modelsPVC.Metadata.Name, "Models PVC name should match")
		assert.Equal(t, "radiocontestwinner-output", outputPVC.Metadata.Name, "Output PVC name should match")

		// Validate deployment references PVCs correctly
		deploymentPVCs := extractPVCNamesFromDeployment(deployment)
		assert.Contains(t, deploymentPVCs, "radiocontestwinner-models", "Deployment should reference models PVC")
		assert.Contains(t, deploymentPVCs, "radiocontestwinner-output", "Deployment should reference output PVC")

		// Validate deployment references ConfigMap correctly
		configMapRef := extractConfigMapNameFromDeployment(deployment)
		assert.Equal(t, expectedConfigMapName, configMapRef, "Deployment should reference correct ConfigMap")
	})
}

// TestManifestConsistency validates consistency across all manifests
func TestManifestConsistency(t *testing.T) {
	t.Run("All manifests have consistent labels and version", func(t *testing.T) {
		// ARRANGE: Expected consistent labels
		expectedVersion := "v3.4"
		expectedAppLabel := "radiocontestwinner"

		// ACT: Load all manifests
		deployment, _ := loadDeploymentManifest()
		service, _ := loadServiceManifest()
		configMap, _ := loadConfigMapManifest()
		secret, _ := loadSecretManifest()
		modelsPVC, _ := loadPVCManifest("pvc-models.yaml")
		outputPVC, _ := loadPVCManifest("pvc-output.yaml")

		// ASSERT: Validate version consistency
		allResources := []struct {
			name     string
			resource interface{}
			metadata ObjectMeta
		}{
			{"Deployment", deployment, deployment.Metadata},
			{"Service", service, service.Metadata},
			{"ConfigMap", configMap, configMap.Metadata},
			{"Secret", secret, secret.Metadata},
			{"Models PVC", modelsPVC, modelsPVC.Metadata},
			{"Output PVC", outputPVC, outputPVC.Metadata},
		}

		for _, resource := range allResources {
			t.Run(fmt.Sprintf("%s has correct version", resource.name), func(t *testing.T) {
				assert.Equal(t, expectedVersion, resource.metadata.Labels["version"],
					"%s should have version %s", resource.name, expectedVersion)
				assert.Equal(t, expectedAppLabel, resource.metadata.Labels["app"],
					"%s should have app label %s", resource.name, expectedAppLabel)
			})
		}
	})
}

// TestResourceConfiguration validates resource limits are appropriate
func TestResourceConfiguration(t *testing.T) {
	t.Run("Resource configuration is appropriate for workload", func(t *testing.T) {
		// ARRANGE: Expected resource configuration
		expectedCPULimit := "1"
		expectedCPURequest := "500m"
		expectedMemoryLimit := "1Gi"
		expectedMemoryRequest := "512Mi"
		expectedModelsStorage := "5Gi"
		expectedOutputStorage := "10Gi"

		// ACT: Load resource specifications
		deployment, err := loadDeploymentManifest()
		assert.NoError(t, err, "Should load deployment manifest")

		modelsPVC, err := loadPVCManifest("pvc-models.yaml")
		assert.NoError(t, err, "Should load models PVC manifest")

		outputPVC, err := loadPVCManifest("pvc-output.yaml")
		assert.NoError(t, err, "Should load output PVC manifest")

		// ASSERT: Validate resource limits
		container := deployment.Spec.Template.Spec.Containers[0]
		resources := container.Resources

		// Validate CPU and memory limits
		assert.Equal(t, expectedCPULimit, resources.Limits.CPU, "CPU limit should be 1")
		assert.Equal(t, expectedCPURequest, resources.Requests.CPU, "CPU request should be 500m")
		assert.Equal(t, expectedMemoryLimit, resources.Limits.Memory, "Memory limit should be 1Gi")
		assert.Equal(t, expectedMemoryRequest, resources.Requests.Memory, "Memory request should be 512Mi")

		// Validate storage sizing
		assert.Equal(t, expectedModelsStorage, modelsPVC.Spec.Resources.Requests.Storage, "Models storage should be 5Gi")
		assert.Equal(t, expectedOutputStorage, outputPVC.Spec.Resources.Requests.Storage, "Output storage should be 10Gi")

		// Validate resource ratio (request should be half of limit for memory)
		assert.Equal(t, "512Mi", resources.Requests.Memory, "Memory request should be appropriate")
	})
}

// TestSecurityConfiguration validates security best practices
func TestSecurityConfiguration(t *testing.T) {
	t.Run("Security configuration follows best practices", func(t *testing.T) {
		// ARRANGE: Expected security configuration
		expectedUserID := int64(1000)
		expectedGroupID := int64(1000)
		expectedReadOnlyRootFilesystem := true
		expectedAllowPrivilegeEscalation := false
		expectedRunAsNonRoot := true

		// ACT: Load security configurations
		deployment, err := loadDeploymentManifest()
		assert.NoError(t, err, "Should load deployment manifest")

		secret, err := loadSecretManifest()
		assert.NoError(t, err, "Should load secret manifest")

		// ASSERT: Validate pod security context
		podSecurityContext := deployment.Spec.Template.Spec.SecurityContext
		assert.NotNil(t, podSecurityContext, "Should have pod security context")
		assert.Equal(t, expectedUserID, *podSecurityContext.RunAsUser, "Should run as non-root user")
		assert.Equal(t, expectedGroupID, *podSecurityContext.RunAsGroup, "Should run as non-root group")
		assert.Equal(t, expectedGroupID, *podSecurityContext.FSGroup, "Should have filesystem group")

		// ASSERT: Validate container security context
		container := deployment.Spec.Template.Spec.Containers[0]
		containerSecurityContext := container.SecurityContext
		assert.NotNil(t, containerSecurityContext, "Should have container security context")
		assert.Equal(t, expectedReadOnlyRootFilesystem, *containerSecurityContext.ReadOnlyRootFilesystem, "Should have read-only root filesystem")
		assert.Equal(t, expectedAllowPrivilegeEscalation, *containerSecurityContext.AllowPrivilegeEscalation, "Should not allow privilege escalation")
		assert.Equal(t, expectedRunAsNonRoot, *containerSecurityContext.RunAsNonRoot, "Should run as non-root")

		// ASSERT: Validate secret uses placeholders (not actual secrets)
		assert.Contains(t, secret.Data, "stream-auth-token", "Should have stream auth token")
		assert.Contains(t, secret.Data["stream-auth-token"], "${", "Should use placeholder format for secrets")
	})
}

// TestHealthCheckConfiguration validates health check configuration
func TestHealthCheckConfiguration(t *testing.T) {
	t.Run("Health check configuration is appropriate", func(t *testing.T) {
		// ARRANGE: Expected health check configuration
		expectedInitialDelaySeconds := int32(30)
		expectedPeriodSeconds := int32(30)
		expectedTimeoutSeconds := int32(10)

		// ACT: Load deployment manifest
		deployment, err := loadDeploymentManifest()
		assert.NoError(t, err, "Should load deployment manifest")

		// ASSERT: Validate liveness probe
		container := deployment.Spec.Template.Spec.Containers[0]
		livenessProbe := container.LivenessProbe
		assert.NotNil(t, livenessProbe, "Should have liveness probe")
		assert.Equal(t, expectedInitialDelaySeconds, livenessProbe.InitialDelaySeconds, "Initial delay should be 30s")
		assert.Equal(t, expectedPeriodSeconds, livenessProbe.PeriodSeconds, "Period should be 30s")
		assert.Equal(t, expectedTimeoutSeconds, livenessProbe.TimeoutSeconds, "Timeout should be 10s")

		// Validate health check command
		assert.Len(t, livenessProbe.Exec.Command, 2, "Should have health check command")
		assert.Equal(t, "/app/radiocontestwinner", livenessProbe.Exec.Command[0], "Should use binary path")
		assert.Equal(t, "--health", livenessProbe.Exec.Command[1], "Should use health flag")

		// Validate readiness probe
		readinessProbe := container.ReadinessProbe
		assert.NotNil(t, readinessProbe, "Should have readiness probe")
		assert.Equal(t, int32(5), readinessProbe.InitialDelaySeconds, "Readiness probe should start faster")
		assert.Equal(t, int32(10), readinessProbe.PeriodSeconds, "Readiness probe should check more frequently")
	})
}

// TestManifestFileExistence validates all required manifest files exist
func TestManifestFileExistence(t *testing.T) {
	t.Run("All required manifest files exist", func(t *testing.T) {
		// ARRANGE: Required manifest files
		requiredFiles := []string{
			"deployment.yaml",
			"service.yaml",
			"configmap.yaml",
			"secret.yaml",
			"pvc-models.yaml",
			"pvc-output.yaml",
		}

		// ACT & ASSERT: Check each file exists
		for _, filename := range requiredFiles {
			t.Run(fmt.Sprintf("%s exists", filename), func(t *testing.T) {
				if _, err := os.Stat(filename); os.IsNotExist(err) {
					t.Errorf("Required manifest file %s does not exist", filename)
				}
			})
		}

		// Additional check: validate all files are valid YAML
		for _, filename := range requiredFiles {
			t.Run(fmt.Sprintf("%s is valid YAML", filename), func(t *testing.T) {
				data, err := os.ReadFile(filename)
				assert.NoError(t, err, "Should be able to read file %s", filename)

				var testStruct interface{}
				err = yaml.Unmarshal(data, &testStruct)
				assert.NoError(t, err, "File %s should be valid YAML", filename)
			})
		}
	})
}

// Helper functions for integration tests

// extractPVCNamesFromDeployment extracts PVC names from deployment volume claims
func extractPVCNamesFromDeployment(deployment *Deployment) []string {
	var pvcNames []string
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.PersistentVolumeClaim != nil {
			pvcNames = append(pvcNames, volume.PersistentVolumeClaim.ClaimName)
		}
	}
	return pvcNames
}

// extractConfigMapNameFromDeployment extracts ConfigMap name from deployment volumes
func extractConfigMapNameFromDeployment(deployment *Deployment) string {
	for _, volume := range deployment.Spec.Template.Spec.Volumes {
		if volume.ConfigMap != nil {
			return volume.ConfigMap.Name
		}
	}
	return ""
}

// getTestFiles returns all test files in the current directory
func getTestFiles() ([]string, error) {
	var testFiles []string

	err := filepath.Walk(".", func(path string, info os.FileInfo, err error) error {
		if err != nil {
			return err
		}

		if !info.IsDir() && strings.HasSuffix(path, "_test.go") {
			testFiles = append(testFiles, path)
		}
		return nil
	})

	return testFiles, err
}