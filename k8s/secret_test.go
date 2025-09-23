package k8s

import (
	"fmt"
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"gopkg.in/yaml.v2"
)

// TestSecretManifest validates the Kubernetes Secret template configuration
func TestSecretManifest(t *testing.T) {
	// Test case: Secret template should have correct configuration
	t.Run("Secret template has correct configuration", func(t *testing.T) {
		// ARRANGE: Expected Secret template configuration
		expectedName := "radiocontestwinner-secrets"
		expectedType := "Opaque"

		// ACT: Read and parse the Secret template manifest
		secret, err := loadSecretManifest()

		// ASSERT: Validate Secret template configuration
		assert.NoError(t, err, "Should load Secret template manifest without errors")
		assert.NotNil(t, secret, "Secret should not be nil")

		// Validate Secret metadata
		assert.Equal(t, expectedName, secret.Metadata.Name, "Secret name should match")
		assert.Contains(t, secret.Metadata.Labels, "app", "Should have app label")
		assert.Equal(t, "radiocontestwinner", secret.Metadata.Labels["app"], "App label should match")
		assert.Equal(t, expectedType, secret.Type, "Secret type should be Opaque")

		// Validate that Secret data contains placeholders (not actual values)
		data := secret.Data
		assert.NotNil(t, data, "Should have secret data")

		// Check for placeholder values that should be replaced
		assert.Contains(t, data, "stream-auth-token", "Should have stream authentication token placeholder")
		assert.Contains(t, data, "api-key", "Should have API key placeholder")
		assert.Contains(t, data, "db-connection-string", "Should have database connection string placeholder")

		// Validate placeholder format
		assert.Equal(t, "${STREAM_AUTH_TOKEN}", data["stream-auth-token"], "Should have placeholder format for stream auth token")
		assert.Equal(t, "${API_KEY}", data["api-key"], "Should have placeholder format for API key")
		assert.Equal(t, "${DB_CONNECTION_STRING}", data["db-connection-string"], "Should have placeholder format for DB connection string")
	})
}

// TestSecretTemplateStructure validates Secret template structure and completeness
func TestSecretTemplateStructure(t *testing.T) {
	t.Run("Secret template has complete structure", func(t *testing.T) {
		// ACT: Read Secret template manifest
		secret, err := loadSecretManifest()

		// ASSERT: Validate structure completeness
		assert.NoError(t, err, "Should load Secret template manifest without errors")
		assert.NotNil(t, secret, "Secret should not be nil")

		// Validate required data fields are present
		expectedSecretFields := []string{
			"stream-auth-token",
			"api-key",
			"db-connection-string",
			"external-service-password",
			"webhook-secret",
		}

		data := secret.Data
		assert.NotNil(t, data, "Should have secret data")

		for _, field := range expectedSecretFields {
			assert.Contains(t, data, field, "Should have secret field: %s", field)

			// All values should be placeholders, not actual secrets
			value := data[field]
			assert.True(t, len(value) > 0, "Field %s should have a placeholder value", field)
			assert.Contains(t, value, "${", "Field %s should use placeholder format", field)
			assert.Contains(t, value, "}", "Field %s should use placeholder format", field)
		}
	})
}

// TestSecretMetadata validates Secret labels and annotations
func TestSecretMetadata(t *testing.T) {
	t.Run("Secret has correct metadata", func(t *testing.T) {
		// ARRANGE: Expected labels and annotations
		expectedLabels := map[string]string{
			"app":         "radiocontestwinner",
			"version":     "v3.4",
			"component":   "secrets",
			"sensitive":   "true",
		}

		expectedAnnotations := map[string]string{
			"description": "Radio Contest Winner application secrets - replace placeholders with actual values",
			"managed-by":  "kubernetes-manual",
			"warning":     "Contains sensitive data - apply appropriate access controls",
		}

		// ACT: Read Secret template manifest
		secret, err := loadSecretManifest()

		// ASSERT: Validate metadata
		assert.NoError(t, err, "Should load Secret template manifest without errors")
		assert.NotNil(t, secret, "Secret should not be nil")

		// Validate labels
		labels := secret.Metadata.Labels
		assert.NotNil(t, labels, "Should have labels")

		for key, expectedValue := range expectedLabels {
			assert.Contains(t, labels, key, "Should have label %s", key)
			assert.Equal(t, expectedValue, labels[key], "Label %s should have correct value", key)
		}

		// Validate annotations
		annotations := secret.Metadata.Annotations
		assert.NotNil(t, annotations, "Should have annotations")

		for key, expectedValue := range expectedAnnotations {
			assert.Contains(t, annotations, key, "Should have annotation %s", key)
			assert.Equal(t, expectedValue, annotations[key], "Annotation %s should have correct value", key)
		}
	})
}

// TestSecretDocumentation validates that the template includes documentation
func TestSecretDocumentation(t *testing.T) {
	t.Run("Secret template includes proper documentation", func(t *testing.T) {
		// ACT: Read Secret template manifest
		secret, err := loadSecretManifest()

		// ASSERT: Validate documentation
		assert.NoError(t, err, "Should load Secret template manifest without errors")
		assert.NotNil(t, secret, "Secret should not be nil")

		// Check for usage documentation in annotations
		annotations := secret.Metadata.Annotations
		assert.Contains(t, annotations, "usage-instructions", "Should have usage instructions")

		usageInstructions := annotations["usage-instructions"]
		assert.Contains(t, usageInstructions, "Replace", "Usage instructions should mention replacing placeholders")
		assert.Contains(t, usageInstructions, "kubectl", "Usage instructions should mention kubectl")
		assert.Contains(t, usageInstructions, "base64", "Usage instructions should mention base64 encoding")

		// Check for security warnings
		assert.Contains(t, annotations, "security-warning", "Should have security warning")

		securityWarning := annotations["security-warning"]
		assert.Contains(t, securityWarning, "confidential", "Security warning should mention confidentiality")
		assert.Contains(t, securityWarning, "access control", "Security warning should mention access control")
	})
}

// loadSecretManifest is a helper function to load the Secret template manifest
func loadSecretManifest() (*Secret, error) {
	// Read the secret.yaml file
	data, err := os.ReadFile("secret.yaml")
	if err != nil {
		return nil, fmt.Errorf("failed to read secret.yaml: %w", err)
	}

	// Parse the YAML
	var secret Secret
	if err := yaml.Unmarshal(data, &secret); err != nil {
		return nil, fmt.Errorf("failed to parse secret.yaml: %w", err)
	}

	return &secret, nil
}

// Secret represents the Kubernetes Secret structure
type Secret struct {
	Metadata  ObjectMeta            `yaml:"metadata" json:"metadata"`
	Type      string                `yaml:"type" json:"type"`
	Data      map[string]string      `yaml:"data" json:"data"`
	StringData map[string]string     `yaml:"stringData" json:"stringData"`
}