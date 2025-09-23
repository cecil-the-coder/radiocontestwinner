package k8s

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// TestDeploymentManifest validates the Kubernetes Deployment manifest configuration
func TestDeploymentManifest(t *testing.T) {
	// Test case: Deployment manifest should have correct configuration
	t.Run("Deployment manifest has correct configuration", func(t *testing.T) {
		// ARRANGE: Expected deployment configuration
		expectedAppName := "radiocontestwinner"
		expectedReplicas := int32(1)
		expectedContainerPort := int32(8080)

		// ACT: Read and parse the deployment manifest
		deployment, err := loadDeploymentManifest()

		// ASSERT: Validate deployment configuration
		assert.NoError(t, err, "Should load deployment manifest without errors")
		assert.NotNil(t, deployment, "Deployment should not be nil")

		// Validate deployment metadata
		assert.Equal(t, expectedAppName, deployment.Metadata.Name, "Deployment name should match")
		assert.Contains(t, deployment.Metadata.Labels, "app", "Should have app label")
		assert.Equal(t, expectedAppName, deployment.Metadata.Labels["app"], "App label should match")

		// Validate deployment spec
		assert.Equal(t, expectedReplicas, deployment.Spec.Replicas, "Should have correct replica count")
		assert.NotNil(t, deployment.Spec.Selector, "Should have selector")
		assert.Equal(t, expectedAppName, deployment.Spec.Selector.MatchLabels["app"], "Selector should match app label")

		// Validate pod template spec
		assert.Len(t, deployment.Spec.Template.Spec.Containers, 1, "Should have exactly one container")
		container := deployment.Spec.Template.Spec.Containers[0]

		assert.Equal(t, expectedAppName, container.Name, "Container name should match")
		assert.Contains(t, container.Image, "radiocontestwinner", "Image should contain app name")
		assert.Len(t, container.Ports, 1, "Should have exactly one port")
		assert.Equal(t, expectedContainerPort, container.Ports[0].ContainerPort, "Container port should match")

		// Validate health check
		assert.NotNil(t, container.LivenessProbe, "Should have liveness probe")
		assert.NotNil(t, container.ReadinessProbe, "Should have readiness probe")
		assert.Equal(t, "/app/radiocontestwinner", container.LivenessProbe.Exec.Command[0], "Liveness probe should use health check")
		assert.Equal(t, "--health", container.LivenessProbe.Exec.Command[1], "Liveness probe should use health flag")

		// Validate resource limits
		assert.NotNil(t, container.Resources.Limits, "Should have resource limits")
		assert.NotNil(t, container.Resources.Requests, "Should have resource requests")

		// Validate security context
		assert.NotNil(t, deployment.Spec.Template.Spec.SecurityContext, "Should have pod security context")
		assert.NotNil(t, container.SecurityContext, "Should have container security context")
	})
}

// TestDeploymentRollingUpdateStrategy validates the rolling update configuration
func TestDeploymentRollingUpdateStrategy(t *testing.T) {
	t.Run("Deployment has correct rolling update strategy", func(t *testing.T) {
		// ARRANGE: Expected rolling update configuration
		expectedMaxUnavailable := "0"
		expectedMaxSurge := "1"

		// ACT: Read deployment manifest
		deployment, err := loadDeploymentManifest()

		// ASSERT: Validate rolling update strategy
		assert.NoError(t, err, "Should load deployment manifest without errors")
		assert.NotNil(t, deployment.Spec.Strategy, "Should have deployment strategy")
		assert.Equal(t, "RollingUpdate", deployment.Spec.Strategy.Type, "Should use rolling update strategy")

		rollingUpdate := deployment.Spec.Strategy.RollingUpdate
		assert.NotNil(t, rollingUpdate, "Should have rolling update configuration")
		assert.Equal(t, expectedMaxUnavailable, rollingUpdate.MaxUnavailable, "MaxUnavailable should be 0")
		assert.Equal(t, expectedMaxSurge, rollingUpdate.MaxSurge, "MaxSurge should be 1")
	})
}

// TestDeploymentResourceLimits validates resource limits and requests
func TestDeploymentResourceLimits(t *testing.T) {
	t.Run("Deployment has correct resource limits", func(t *testing.T) {
		// ARRANGE: Expected resource configuration based on Story 3.3 performance data
		expectedMemoryLimit := "1Gi"
		expectedMemoryRequest := "512Mi"
		expectedCPULimit := "1"
		expectedCPURequest := "500m"

		// ACT: Read deployment manifest
		deployment, err := loadDeploymentManifest()

		// ASSERT: Validate resource configuration
		assert.NoError(t, err, "Should load deployment manifest without errors")
		assert.Len(t, deployment.Spec.Template.Spec.Containers, 1, "Should have exactly one container")

		container := deployment.Spec.Template.Spec.Containers[0]
		resources := container.Resources

		// Validate memory limits
		assert.Equal(t, expectedMemoryLimit, resources.Limits.Memory, "Memory limit should be 1Gi")
		assert.Equal(t, expectedMemoryRequest, resources.Requests.Memory, "Memory request should be 512Mi")

		// Validate CPU limits
		assert.Equal(t, expectedCPULimit, resources.Limits.CPU, "CPU limit should be 1")
		assert.Equal(t, expectedCPURequest, resources.Requests.CPU, "CPU request should be 500m")
	})
}

// TestDeploymentSecurityContext validates security context configuration
func TestDeploymentSecurityContext(t *testing.T) {
	t.Run("Deployment has correct security context", func(t *testing.T) {
		// ARRANGE: Expected security configuration
		expectedUserID := int64(1000)
		expectedGroupID := int64(1000)
		expectedFSGroup := int64(1000)
		expectedReadOnlyRootFilesystem := true
		expectedAllowPrivilegeEscalation := false
		expectedRunAsNonRoot := true

		// ACT: Read deployment manifest
		deployment, err := loadDeploymentManifest()

		// ASSERT: Validate security configuration
		assert.NoError(t, err, "Should load deployment manifest without errors")

		podSecurityContext := deployment.Spec.Template.Spec.SecurityContext
		container := deployment.Spec.Template.Spec.Containers[0]
		containerSecurityContext := container.SecurityContext

		// Validate pod security context
		assert.NotNil(t, podSecurityContext, "Should have pod security context")
		assert.Equal(t, expectedUserID, *podSecurityContext.RunAsUser, "Should run as non-root user (1000)")
		assert.Equal(t, expectedGroupID, *podSecurityContext.RunAsGroup, "Should run as non-root group (1000)")
		assert.Equal(t, expectedFSGroup, *podSecurityContext.FSGroup, "Should have filesystem group (1000)")

		// Validate container security context
		assert.NotNil(t, containerSecurityContext, "Should have container security context")
		assert.Equal(t, expectedReadOnlyRootFilesystem, *containerSecurityContext.ReadOnlyRootFilesystem, "Should have read-only root filesystem")
		assert.Equal(t, expectedAllowPrivilegeEscalation, *containerSecurityContext.AllowPrivilegeEscalation, "Should not allow privilege escalation")
		assert.Equal(t, expectedRunAsNonRoot, *containerSecurityContext.RunAsNonRoot, "Should run as non-root")
	})
}

// loadDeploymentManifest is a helper function to load the deployment manifest
func loadDeploymentManifest() (*Deployment, error) {
	// Read the deployment.yaml file
	data, err := os.ReadFile("deployment.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read deployment.yaml: %w", err)
	}

	// Parse the YAML
	var deployment Deployment
	if err := yaml.Unmarshal(data, &deployment); err != nil {
		return nil, fmt.Errorf("failed to parse deployment.yaml: %w", err)
	}

	return &deployment, nil
}

// Deployment represents the Kubernetes Deployment structure
type Deployment struct {
	Metadata ObjectMeta        `yaml:"metadata" json:"metadata"`
	Spec     DeploymentSpec    `yaml:"spec" json:"spec"`
}

// DeploymentSpec represents the Kubernetes Deployment specification
type DeploymentSpec struct {
	Replicas int32            `yaml:"replicas" json:"replicas"`
	Selector LabelSelector    `yaml:"selector" json:"selector"`
	Template PodTemplateSpec  `yaml:"template" json:"template"`
	Strategy DeploymentStrategy `yaml:"strategy" json:"strategy"`
}

// DeploymentStrategy represents the deployment strategy
type DeploymentStrategy struct {
	Type           string               `yaml:"type" json:"type"`
	RollingUpdate  *RollingUpdateConfig `yaml:"rollingUpdate" json:"rollingUpdate"`
}

// RollingUpdateConfig represents the rolling update configuration
type RollingUpdateConfig struct {
	MaxUnavailable string `yaml:"maxUnavailable" json:"maxUnavailable"`
	MaxSurge       string `yaml:"maxSurge" json:"maxSurge"`
}

// PodTemplateSpec represents the pod template specification
type PodTemplateSpec struct {
	Metadata ObjectMeta     `yaml:"metadata" json:"metadata"`
	Spec     PodSpec        `yaml:"spec" json:"spec"`
}

// PodSpec represents the pod specification
type PodSpec struct {
	Containers      []Container         `yaml:"containers" json:"containers"`
	SecurityContext *PodSecurityContext `yaml:"securityContext" json:"securityContext"`
	Volumes         []Volume            `yaml:"volumes" json:"volumes"`
}

// Volume represents a pod volume
type Volume struct {
	Name               string            `yaml:"name" json:"name"`
	ConfigMap          *ConfigMapVolume  `yaml:"configMap" json:"configMap"`
	PersistentVolumeClaim *PVCVolume      `yaml:"persistentVolumeClaim" json:"persistentVolumeClaim"`
	EmptyDir           *EmptyDirVolume   `yaml:"emptyDir" json:"emptyDir"`
}

// ConfigMapVolume represents a ConfigMap volume source
type ConfigMapVolume struct {
	Name string `yaml:"name" json:"name"`
}

// PVCVolume represents a PersistentVolumeClaim volume source
type PVCVolume struct {
	ClaimName string `yaml:"claimName" json:"claimName"`
}

// EmptyDirVolume represents an emptyDir volume source
type EmptyDirVolume struct {
	Medium string `yaml:"medium" json:"medium"`
}

// Container represents a container specification
type Container struct {
	Name            string               `yaml:"name" json:"name"`
	Image           string               `yaml:"image" json:"image"`
	Ports           []ContainerPort      `yaml:"ports" json:"ports"`
	Resources       ResourceRequirements `yaml:"resources" json:"resources"`
	LivenessProbe   *Probe               `yaml:"livenessProbe" json:"livenessProbe"`
	ReadinessProbe  *Probe               `yaml:"readinessProbe" json:"readinessProbe"`
	SecurityContext *ContainerSecurityContext `yaml:"securityContext" json:"securityContext"`
}

// ContainerPort represents a container port
type ContainerPort struct {
	ContainerPort int32 `yaml:"containerPort" json:"containerPort"`
	Name          string `yaml:"name" json:"name"`
	Protocol      string `yaml:"protocol" json:"protocol"`
}

// ResourceRequirements represents resource requirements
type ResourceRequirements struct {
	Limits   ResourceList `yaml:"limits" json:"limits"`
	Requests ResourceList `yaml:"requests" json:"requests"`
}

// ResourceList represents resource limits/requests
type ResourceList struct {
	Memory string `yaml:"memory" json:"memory"`
	CPU    string `yaml:"cpu" json:"cpu"`
}

// Probe represents a health check probe
type Probe struct {
	Exec             *ExecAction `yaml:"exec" json:"exec"`
	InitialDelaySeconds int32      `yaml:"initialDelaySeconds" json:"initialDelaySeconds"`
	PeriodSeconds     int32      `yaml:"periodSeconds" json:"periodSeconds"`
	TimeoutSeconds    int32      `yaml:"timeoutSeconds" json:"timeoutSeconds"`
	FailureThreshold  int32      `yaml:"failureThreshold" json:"failureThreshold"`
}

// ExecAction represents an exec action
type ExecAction struct {
	Command []string `yaml:"command" json:"command"`
}

// ContainerSecurityContext represents container security context
type ContainerSecurityContext struct {
	ReadOnlyRootFilesystem   *bool   `yaml:"readOnlyRootFilesystem" json:"readOnlyRootFilesystem"`
	AllowPrivilegeEscalation *bool   `yaml:"allowPrivilegeEscalation" json:"allowPrivilegeEscalation"`
	RunAsNonRoot             *bool   `yaml:"runAsNonRoot" json:"runAsNonRoot"`
	Capabilities             *CapDef `yaml:"capabilities" json:"capabilities"`
}

// CapDef represents capabilities definition
type CapDef struct {
	Drop []string `yaml:"drop" json:"drop"`
}

// PodSecurityContext represents pod security context
type PodSecurityContext struct {
	RunAsUser  *int64 `yaml:"runAsUser" json:"runAsUser"`
	RunAsGroup *int64 `yaml:"runAsGroup" json:"runAsGroup"`
	FSGroup    *int64 `yaml:"fsGroup" json:"fsGroup"`
}

// LabelSelector represents a label selector
type LabelSelector struct {
	MatchLabels map[string]string `yaml:"matchLabels" json:"matchLabels"`
}

// ObjectMeta represents object metadata
type ObjectMeta struct {
	Name        string            `yaml:"name" json:"name"`
	Labels      map[string]string `yaml:"labels" json:"labels"`
	Annotations map[string]string `yaml:"annotations" json:"annotations"`
}