# Fyve CLI

A command-line tool for the Fyve App Platform that helps deploy Docker-based applications to Kubernetes with auto-scaling capabilities.

## Features

- Deploy Docker-based applications to Kubernetes (primary deployment target)
- Auto-scale applications to and from zero for efficient resource utilization
- Make applications instantly accessible with custom domains using the `fyve publish` command
- Secure authentication with the Fyve App Platform
- Build NextJS and other Docker-based applications
- Legacy support for Docker host deployment (not intended for future use)
- Handle secrets using AWS Systems Manager Parameter Store
- Automatically use default Dockerfile if one doesn't exist in the project
- Automatically create ECR repositories if they don't exist

## Installation

```bash
curl -sfL https://github.com/Fyve-Labs/fyve-cli/raw/main/install.sh | bash
```

The script will install fyve at **/usr/local/bin/fyve**

## Usage

### Nextjs config requires

It is required Next.js to produce `standalone` build to deploy using this tool.

```typescript
import type { NextConfig } from 'next'

const nextConfig: NextConfig = {
  output: 'standalone',
}

export default nextConfig
```

### Basic usage

```bash
# Login
fyve login

# Deploy using configuration from fyve.yaml
fyve deploy

# Deploy from an image
fyve deploy --image ghcr.io/traefik/whoami:latest --port 80

# Publish the app. DNS setup will be done from this step
fyve publish

# Publish with other custom domain
fyve publish --domain cool-app.fyve.dev

# Specify a different config file
fyve deploy --config custom-config.yaml

# Deploy to a remote Docker host
fyve deploy --docker --docker-host tcp://remote-host:2375

# List all apps
fyve list
```

### Configuration

Fyve CLI uses YAML configuration files. Here's an example:

```yaml
app: app-name
env:
  AWS_REGION: us-east-1
  AWS_S3_BUCKET: example
  APP_URL: https://examples.com
  DATABASE_URL: secret:/app-name/DATABASE_URL
```

### Secrets

Secrets are retrieved from AWS Systems Manager Parameter Store. To reference a secret, use the format:

```
secret:/app-name/environment/SECRET_NAME
```

### Dockerfile

Fyve automatically uses a default Dockerfile optimized for NextJS apps if one doesn't exist in your project. If you want to customize the build process, simply add your own `Dockerfile` to your project's root directory.

### AWS ECR Integration

When deploying, Fyve automatically:
1. Creates the ECR repository if it doesn't exist
2. Logs in to ECR using secure authentication
3. Tags and pushes your Docker image

Fyve uses AWS SDK for Go v2 for direct integration with AWS services, providing improved error handling, context support, and better concurrency. This eliminates the need for the AWS CLI to be installed on your system for ECR operations.

You need to have AWS credentials configured in the standard AWS SDK locations (environment variables, ~/.aws/credentials, etc.) with appropriate permissions to create and access ECR repositories.

### Traefik Integration

When deploying to Docker, Fyve automatically configures Traefik labels for your container:

- Enables Traefik routing
- Sets up a domain name `<app-name>.fyve.dev`
- Configures TLS with automatic certificate generation
- Routes traffic through the WebSecure entrypoint
- Maps to internal port 3000
- Attaches containers to the "public" network for proper routing

This allows your application to be immediately accessible via HTTPS with proper routing and load balancing, without any additional configuration. Note that container ports are intentionally not exposed to the host, as Traefik handles routing directly through Docker networks, allowing multiple containers to run on the same host without port conflicts.

## License

MIT