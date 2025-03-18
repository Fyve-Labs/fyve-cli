package secrets

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/service/ssm"
)

// SSMManager handles retrieving secrets from AWS Systems Manager Parameter Store
type SSMManager struct {
	ssmClient *ssm.Client
}

// NewSSMManager creates a new AWS SSM Parameter Store manager
func NewSSMManager(client *ssm.Client) (*SSMManager, error) {
	return &SSMManager{
		ssmClient: client,
	}, nil
}

// GetSecret retrieves a secret from SSM Parameter Store
func (m *SSMManager) GetSecret(secretRef string, environment string) (string, error) {
	ctx := context.Background()

	// Parse secret reference, expected format: secret:/app-name/{environment}/SECRET_NAME
	if !strings.HasPrefix(secretRef, "secret:") {
		return "", fmt.Errorf("invalid secret reference format: %s", secretRef)
	}

	// Extract parameter path
	paramPath := strings.TrimPrefix(secretRef, "secret:")

	// Replace {environment} placeholder with actual environment name
	paramPath = strings.ReplaceAll(paramPath, "{environment}", environment)

	// Create pointer to boolean for WithDecryption field
	decrypt := true

	// Get parameter from SSM
	param, err := m.ssmClient.GetParameter(ctx, &ssm.GetParameterInput{
		Name:           &paramPath,
		WithDecryption: &decrypt,
	})

	if err != nil {
		return "", fmt.Errorf("failed to get parameter from SSM: %w", err)
	}

	return *param.Parameter.Value, nil
}

// ProcessSecretRefs resolves secret references in environment variables
func (m *SSMManager) ProcessSecretRefs(env map[string]string, environment string) (map[string]string, error) {
	result := make(map[string]string)

	for key, val := range env {
		if strings.HasPrefix(val, "secret:") {
			secretVal, err := m.GetSecret(val, environment)
			if err != nil {
				return nil, fmt.Errorf("failed to resolve secret for %s: %w", key, err)
			}
			result[key] = secretVal
		} else {
			result[key] = val
		}
	}

	return result, nil
}
