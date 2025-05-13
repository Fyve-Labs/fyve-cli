package config

import (
	"fmt"
	"io"
	"k8s.io/client-go/tools/clientcmd"
	"k8s.io/client-go/tools/clientcmd/api"
	"net/http"
	"os"
	"path/filepath"
)

const defaultKubeconfigTemplate = "https://raw.githubusercontent.com/Fyve-Labs/fyve-cli/main/docs/kubeconfig/kubeconfig.tpl"

func LoadKubeconfig() (string, error) {
	homeDir, err := os.UserHomeDir()
	if err != nil {
		return "", err
	}

	// Create directory if it doesn't exist
	fyveDirPath := filepath.Join(homeDir, ".fyve")
	if err = os.MkdirAll(fyveDirPath, 0755); err != nil {
		return "", fmt.Errorf("error creating directory: %v", err)
	}

	// Path to save the kubeconfig file
	kubeconfigPath := filepath.Join(fyveDirPath, "kubeconfig")

	// Check if the file already exists
	if _, err = os.Stat(kubeconfigPath); os.IsNotExist(err) {
		templateURL := os.Getenv("FYVE_KUBECONFIG_TEMPLATE")
		if templateURL == "" {
			templateURL = defaultKubeconfigTemplate
		}

		// Download the kubeconfig template
		resp, err := http.Get(templateURL)
		if err != nil {
			return "", fmt.Errorf("error downloading kubeconfig template: %v", err)
		}
		defer resp.Body.Close()

		// Create the file
		file, err := os.Create(kubeconfigPath)
		if err != nil {
			return "", fmt.Errorf("error creating kubeconfig file: %v", err)
		}
		defer file.Close()

		// Copy the response body to the file
		_, err = io.Copy(file, resp.Body)
		if err != nil {
			return "", fmt.Errorf("error writing to kubeconfig file: %v", err)
		}
	}

	kubeconfig, err := clientcmd.LoadFromFile(kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("could not load %s: %w", kubeconfigPath, err)
	}

	authConfig, err := LoadAuthConfig()
	if err != nil {
		return "", fmt.Errorf("error loading auth config: %w. Run \"fyve login\" to fix this issue and try again", err)
	}

	if kubeconfig.CurrentContext == "" {
		kubeconfig.CurrentContext = "oidc"
	}

	context := kubeconfig.Contexts[kubeconfig.CurrentContext]
	token := authConfig.IDToken
	if token == "" {
		token = authConfig.AccessToken
	}

	if token == "" {
		return "", fmt.Errorf("could not find token in auth config. Run \"fyve login\" to fix this issue and try again")
	}

	kubeconfig.AuthInfos[context.AuthInfo] = &api.AuthInfo{
		Token: token,
	}

	err = clientcmd.WriteToFile(*kubeconfig, kubeconfigPath)
	if err != nil {
		return "", fmt.Errorf("error writing to kubeconfig file: %w", err)
	}

	return kubeconfigPath, nil
}
