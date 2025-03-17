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
```

### Configuration

Fyve CLI uses YAML configuration files. Here's an example:

```yaml
app: app-name
env:
  AWS_REGION: us-east-1
  AWS_S3_BUCKET: example
  APP_URL: https://examples.com
  DATABASE_URL: secret:app-name/environment/DATABASE_URL
```

### Secrets

Secrets are retrieved from AWS Systems Manager Parameter Store. To reference a secret, use the format:

```
secret:app-name/environment/SECRET_NAME
```

### Dockerfile

Fyve automatically uses a default Dockerfile optimized for NextJS apps if one doesn't exist in your project. If you want to customize the build process, simply add your own `Dockerfile` to your project's root directory.

### AWS ECR Integration

When deploying, Fyve automatically:
1. Creates the ECR repository if it doesn't exist
2. Logs in to ECR
3. Tags and pushes your Docker image

You need to have the AWS CLI configured with appropriate permissions to create and access ECR repositories.

## Command Line Options

```
fyve deploy [app-name] [flags]

Flags:
  -c, --config string         Path to configuration file (default "fyve.yaml")
  -d, --docker-host string    Remote Docker host URL (default "tcp://docker-host:2375")
  -e, --environment string    Deployment environment (prod, staging, dev, test, preview) (default "prod")
  -h, --help                  help for deploy
  -i, --image-prefix string   Prefix for Docker image names (default "fyve-")
  -p, --port string           Port to expose on the host (default "3000")
  -r, --registry string       ECR registry URL (default "aws_account_id.dkr.ecr.region.amazonaws.com")
```

## License

MIT 