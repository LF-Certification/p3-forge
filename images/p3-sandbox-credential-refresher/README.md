# P3 Sandbox ECR Credential Refresher

This image provides the runtime environment for the ECR credential refresher CronJob that maintains Docker authentication credentials for accessing AWS ECR pull-through cache repositories.

## Contents

- AWS CLI v2.17.47
- kubectl v1.31.0

## Usage

This image is used by the ECR credential refresher CronJob in the P3 platform to:
1. Authenticate to AWS using OIDC via a service account
2. Retrieve ECR authentication tokens using `aws ecr get-login-password`
3. Create/update a Kubernetes secret with Docker registry credentials
