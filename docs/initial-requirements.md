Build a command line application using GO. The CLI is named "fyve", its main feature is to build and deploy NextJS application to a remote docker host. More details:
- The command will be `fyve deploy` with optional argument options:
    * app name
    * environment: valid values are: prod, staging, dev, test, preview. Default value is `prod`
- CLI takes input config file as yaml format, for example:
  app: app-name
  env:
  AWS_REGION: us-east-1
  AWS_S3_BUCKET: example
  APP_URL: https://examples.com
  DATABASE_URL: secret:/app-name/{environment}/DATABASE_URL
- App name can be overriden from command line arguments
- Secrets will be backed by AWS Systems Manager Parameter Store
- CLI builds NEXTJS project using dockerfile, push image to AWS ECR. If the Dockerfile does not exist within NextJS project, it will fall back to use embed Dockerfile using nextjs.Dockerfile ✅
- CLI create ECR repository if it does not exist ✅
- CLI connects to remote docker host to deploy app with newly built image
- The project must be extensible for further support deployment other project types, not just NextJs
- If enviromnet is prod, attach these labels to docker deployment: ✅
    - "traefik.enable=true"
    - "traefik.http.routers.app-name.rule=Host(`app-name.fyve.dev`)"
    - "traefik.http.routers.app-name.tls.certresolver=default"
    - "traefik.http.services.app-name.loadbalancer.server.port=3000"
    - "traefik.http.routers.app-name.entrypoints=websecure"
- If environment is prod, attach docker deployment to network "public" ✅
- Use AWS sdk v2 to integrate AWS services