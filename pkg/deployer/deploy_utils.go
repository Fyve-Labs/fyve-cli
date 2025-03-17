package deployer

import (
	"context"
	"fmt"
	"strings"

	"github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/sts"
)

// ReplaceAWSAccountID replaces AWS account ID placeholders in the registry URL
func ReplaceAWSAccountID(ctx context.Context, registry, awsRegion string) (string, error) {
	// Check if replacement is needed
	if !strings.Contains(registry, "{aws_account_id}") && !strings.Contains(registry, "aws_account_id") {
		return registry, nil
	}

	// Load AWS configuration
	cfg, err := config.LoadDefaultConfig(ctx, config.WithRegion(awsRegion))
	if err != nil {
		return "", fmt.Errorf("failed to load AWS config for region %s: %w", awsRegion, err)
	}

	// Get AWS account ID
	stsClient := sts.NewFromConfig(cfg)
	identity, err := stsClient.GetCallerIdentity(ctx, &sts.GetCallerIdentityInput{})
	if err != nil {
		return "", fmt.Errorf("failed to get AWS account ID using region %s. Make sure you have valid AWS credentials configured: %w", awsRegion, err)
	}

	accountID := *identity.Account
	// Replace both formats
	registry = strings.Replace(registry, "{aws_account_id}", accountID, 1)
	registry = strings.Replace(registry, "aws_account_id", accountID, 1)

	return registry, nil
}
