# Fyve CLI

A command-line tool for building and deploying NextJS applications to remote Docker hosts.

## Features

- Build NextJS applications using Docker
- Push Docker images to AWS ECR
- Deploy to remote Docker hosts
- Handle secrets using AWS Systems Manager Parameter Store (using AWS SDK Go v2)
- Support for different environments (prod, staging, dev, test, preview)
- Extensible architecture for future project types
- Automatically use default Dockerfile if one doesn't exist in the project
- Automatically create ECR repositories if they don't exist

## Installation

```bash
# Clone the repository
git clone https://github.com/fyve-labs/fyve-cli.git

# Build the application
cd fyve-cli
go build -o fyve

# Move to a directory in your PATH (optional)
mv fyve /usr/local/bin/
```

## Usage

### Basic usage

```bash
# Deploy using configuration from fyve.yaml
fyve deploy

# Specify an app name to override the one in config
fyve deploy my-app

# Specify an environment
fyve deploy --environment staging

# Specify a different config file
fyve deploy --config custom-config.yaml

# Deploy to a remote Docker host
fyve deploy --docker-host tcp://remote-host:2375
```

By default, Fyve will deploy to your local Docker daemon if no `--docker-host` is specified. This is convenient for local development and testing. When you're ready to deploy to a remote environment, simply use the `--docker-host` flag with the appropriate connection string.

### Configuration

Fyve CLI uses YAML configuration files. Here's an example:

```yaml
app: app-name
env:
  AWS_REGION: us-east-1
  AWS_S3_BUCKET: example
  APP_URL: https://examples.com
  DATABASE_URL: secret:/app-name/environment/DATABASE_URL
```

### Secrets

Secrets are retrieved from AWS Systems Manager Parameter Store. To reference a secret, use the format:

```
secret:/app-name/environment/SECRET_NAME
```

You can also use the `{environment}` placeholder in your secret paths, which will be automatically replaced with the current deployment environment:

```
secret:/app-name/{environment}/SECRET_NAME
```

This makes it easy to maintain environment-specific secrets without hardcoding environment names in your configuration file.

### Dockerfile

Fyve automatically uses a default Dockerfile optimized for NextJS apps if one doesn't exist in your project. If you want to customize the build process, simply add your own `Dockerfile` to your project's root directory.

### AWS ECR Integration

When deploying, Fyve automatically:
1. Creates the ECR repository if it doesn't exist using AWS SDK v2
2. Logs in to ECR using secure authentication via AWS SDK v2
3. Tags and pushes your Docker image

For production environments, the image tag will be `latest`, while other environments (staging, dev, test, preview) will use the environment name as the image tag (e.g., `fyve-myapp:staging`). This helps with tracking which version is deployed to each environment.

Fyve uses AWS SDK for Go v2 for direct integration with AWS services, providing improved error handling, context support, and better concurrency. This eliminates the need for the AWS CLI to be installed on your system for ECR operations.

You need to have AWS credentials configured in the standard AWS SDK locations (environment variables, ~/.aws/credentials, etc.) with appropriate permissions to create and access ECR repositories.

#### Registry URL Format

The default registry URL format is `{aws_account_id}.dkr.ecr.{region}.amazonaws.com`. Fyve will automatically:

1. Replace `{region}` with the region from your configuration (using `AWS_REGION` from your environment variables or config file)
2. Replace `{aws_account_id}` with your AWS account ID (retrieved using your AWS credentials)

If you want to use a specific ECR registry, you can use the `--registry` flag to provide the full ECR URL, e.g.:
```
fyve deploy --registry 123456789012.dkr.ecr.us-east-1.amazonaws.com
```

**Important**: Valid AWS credentials must be configured in your environment for Fyve to successfully connect to AWS services. Make sure you have the AWS CLI configured or appropriate environment variables set.

### Traefik Integration

When deploying to the production environment, Fyve automatically configures Traefik labels for your container:

- Enables Traefik routing
- Sets up a domain name `<app-name>.fyve.dev`
- Configures TLS with automatic certificate generation
- Routes traffic through the WebSecure entrypoint
- Maps to internal port 3000
- Attaches containers to the "public" network for proper routing

This allows your application to be immediately accessible via HTTPS with proper routing and load balancing, without any additional configuration. Note that container ports are intentionally not exposed to the host, as Traefik handles routing directly through Docker networks, allowing multiple containers to run on the same host without port conflicts.

## Command Line Options

```
fyve deploy [app-name] [flags]

Flags:
  -c, --config string         Path to configuration file (default "fyve.yaml")
  -d, --docker-host string    Remote Docker host URL (if not specified, uses local Docker daemon)
  -e, --environment string    Deployment environment (prod, staging, dev, test, preview) (default "prod")
  -h, --help                  help for deploy
  -i, --image-prefix string   Prefix for Docker image names (default "fyve-")
  -r, --registry string       ECR registry URL (default "aws_account_id.dkr.ecr.region.amazonaws.com")
      --platform string       Target platform for Docker build (default "linux/amd64")
```

## License

MIT