package config

import (
	"context"
	"encoding/base64"
	"fmt"
	"github.com/aws/aws-sdk-go-v2/aws"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"github.com/aws/aws-sdk-go-v2/service/ecr/types"
	"os"
	"os/exec"
	"strings"
)

type Build struct {
	appName       string
	registry      string
	repositoryUri string // format 209479271613.dkr.ecr.us-east-1.amazonaws.com/fyve/fyve-learn
	environment   string
	image         string `yaml:"image"`
}

func (b *Build) GetRepositoryName() string {
	name := b.appName
	if !strings.HasPrefix(name, "fyve-") {
		name = "fyve-" + name
	}

	return fmt.Sprintf("fyve/%s", name)
}

// GetImage return full image url
func (b *Build) GetImage() string {
	if len(b.image) > 0 {
		return b.image
	}

	gitSHA := os.Getenv("GITHUB_SHA")
	if val := os.Getenv("IMAGE_TAG"); val != "" {
		b.image = fmt.Sprintf("%s:%s", b.repositoryUri, val)
	} else if gitSHA != "" {
		shortCommit := gitSHA[:7]
		b.image = fmt.Sprintf("%s:%s", b.repositoryUri, "sha-"+shortCommit)
	} else {
		b.image = fmt.Sprintf("%s:%s", b.repositoryUri, "latest")
	}

	return b.image
}

func (b *Build) EnsureECRRepositoryExists(ctx context.Context, client *ecr.Client) error {
	fmt.Println("Ensuring ECR repository exists...")
	repositoryName := b.GetRepositoryName()
	out, err := client.DescribeRepositories(ctx, &ecr.DescribeRepositoriesInput{
		RepositoryNames: []string{repositoryName},
	})

	if err == nil {
		b.repositoryUri = *out.Repositories[0].RepositoryUri
		return nil
	}

	// Repository doesn't exist, create it
	fmt.Printf("Creating ECR repository '%s'...\n", repositoryName)

	repo, err := client.CreateRepository(ctx, &ecr.CreateRepositoryInput{
		RepositoryName:     aws.String(repositoryName),
		ImageTagMutability: types.ImageTagMutabilityMutable,
		ImageScanningConfiguration: &types.ImageScanningConfiguration{
			ScanOnPush: false,
		},
	})

	if err != nil {
		return fmt.Errorf("failed to create ECR repository: %w", err)
	}

	b.repositoryUri = *repo.Repository.RepositoryUri

	return nil
}

func (b *Build) ECRLogin(ctx context.Context, client *ecr.Client) error {
	fmt.Println("Authenticating with AWS ECR...")

	output, err := client.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return fmt.Errorf("failed to get ECR authorization token: %w", err)
	}

	if len(output.AuthorizationData) == 0 {
		return fmt.Errorf("no authorization data returned")
	}

	authData := output.AuthorizationData[0]
	authToken := *authData.AuthorizationToken
	b.registry = strings.TrimPrefix(*authData.ProxyEndpoint, "https://")

	// Decode the token
	decodedToken, err := base64.StdEncoding.DecodeString(authToken)
	if err != nil {
		return fmt.Errorf("failed to decode authorization token: %w", err)
	}

	// Format is "username:password"
	tokenParts := strings.Split(string(decodedToken), ":")
	if len(tokenParts) != 2 {
		return fmt.Errorf("invalid token format")
	}

	username := tokenParts[0]
	password := tokenParts[1]

	// Login with docker
	dockerLoginCmd := exec.Command("docker", "login", "--username", username, "--password-stdin", b.registry)
	dockerLoginCmd.Stdin = strings.NewReader(string(password))

	_ = dockerLoginCmd.Run()

	return nil
}

func (b *Build) ECRLogout() {
	dockerLogoutCmd := exec.Command("docker", "logout", b.registry)
	if err := dockerLogoutCmd.Run(); err != nil {
		fmt.Printf("failed to run docker logout: %v", err)
	}
}
