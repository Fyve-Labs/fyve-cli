package images

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	awsconfig "github.com/aws/aws-sdk-go-v2/config"
	"github.com/aws/aws-sdk-go-v2/service/ecr"
	"strings"
)

type RegistryClient struct {
	client *ecr.Client
}

type authHeader struct {
	Username      string `json:"username"`
	Password      string `json:"password"`
	ServerAddress string `json:"serveraddress"`
}

func NewRegistryClient() (*RegistryClient, error) {
	awsConfig, err := awsconfig.LoadDefaultConfig(context.TODO(), awsconfig.WithRegion("us-east-1"))
	if err != nil {
		return nil, fmt.Errorf("load AWS credentials error: %w", err)
	}

	client := ecr.NewFromConfig(awsConfig)
	registry := &RegistryClient{
		client: client,
	}

	return registry, nil
}

func (r *RegistryClient) EncodedRegistryAuth(ctx context.Context, img Image) (header string, err error) {
	if !strings.Contains(img.Domain, "ecr") {
		return "", nil
	}

	output, err := r.client.GetAuthorizationToken(ctx, &ecr.GetAuthorizationTokenInput{})
	if err != nil {
		return
	}

	if len(output.AuthorizationData) == 0 {
		return "", fmt.Errorf("no authorization data returned")
	}

	authData := output.AuthorizationData[0]
	authToken := *authData.AuthorizationToken

	// Decode the token
	decodedToken, err := base64.StdEncoding.DecodeString(authToken)
	if err != nil {
		return
	}

	// Format is "username:password"
	tokenParts := strings.Split(string(decodedToken), ":")
	if len(tokenParts) != 2 {
		return "", fmt.Errorf("invalid token format")
	}

	authHeader := authHeader{
		ServerAddress: *authData.ProxyEndpoint,
		Username:      tokenParts[0],
		Password:      tokenParts[1],
	}

	headerData, err := json.Marshal(authHeader)
	if err != nil {
		return
	}

	header = base64.StdEncoding.EncodeToString(headerData)

	return
}
