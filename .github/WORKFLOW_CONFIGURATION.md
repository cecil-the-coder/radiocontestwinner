# CI/CD Workflow Configuration Guide

## Overview

This document describes the configuration requirements for the Radio Contest Winner CI/CD pipeline implemented in `.github/workflows/ci-cd.yml`.

## Required Secrets Configuration

### GitHub Repository Secrets

Configure the following secrets in your GitHub repository settings (`Settings > Secrets and variables > Actions`):

#### Container Registry Access (Required)
- **`GITHUB_TOKEN`**: Automatically provided by GitHub Actions (no manual configuration needed)
  - Used for: Pushing Docker images to GitHub Container Registry (ghcr.io)
  - Permissions: Automatically granted for repository

#### Optional: External Registry Secrets
If using external container registries, configure:

- **`DOCKER_REGISTRY_URL`**: URL of external Docker registry
- **`DOCKER_REGISTRY_USERNAME`**: Username for external registry
- **`DOCKER_REGISTRY_PASSWORD`**: Password/token for external registry

#### Deployment Secrets (Environment-specific)
Configure in GitHub Environment settings (`Settings > Environments`):

**Staging Environment:**
- **`STAGING_DEPLOY_TOKEN`**: Authentication token for staging deployment
- **`STAGING_CLUSTER_URL`**: Kubernetes cluster URL for staging
- **`STAGING_NAMESPACE`**: Kubernetes namespace for staging deployment

**Production Environment:**
- **`PRODUCTION_DEPLOY_TOKEN`**: Authentication token for production deployment
- **`PRODUCTION_CLUSTER_URL`**: Kubernetes cluster URL for production
- **`PRODUCTION_NAMESPACE`**: Kubernetes namespace for production deployment

## Environment Variables

### Built-in Variables (Pre-configured)
```yaml
REGISTRY: ghcr.io                                    # GitHub Container Registry
IMAGE_NAME: ${{ github.repository }}/radiocontestwinner  # Image name with repo prefix
DOCKER_BUILDKIT: 1                                   # Enable Docker BuildKit
```

### Runtime Variables (Automatically Generated)
- **`GITHUB_SHA`**: Git commit SHA for image tagging
- **`GITHUB_REF`**: Git reference (branch/tag) for conditional logic
- **`GITHUB_ACTOR`**: GitHub username for registry authentication

## GitHub Environment Configuration

### Staging Environment
```yaml
Name: staging
Required reviewers: 0 (optional)
Wait timer: 0 minutes
Deployment branches: main, develop
```

### Production Environment
```yaml
Name: production
Required reviewers: 1+ (recommended)
Wait timer: 5 minutes (recommended)
Deployment branches: main only
```

## Workflow Permissions

Ensure the following permissions are granted to `GITHUB_TOKEN`:

```yaml
permissions:
  contents: read
  packages: write
  security-events: write
  actions: read
```

## Branch Protection Rules

Recommended branch protection for `main`:

- ✅ Require a pull request before merging
- ✅ Require status checks to pass before merging
  - Required checks: `test-and-build`
  - Required checks: `security-scan`
- ✅ Require conversation resolution before merging
- ✅ Require signed commits (optional)
- ✅ Include administrators

## Registry Configuration

### GitHub Container Registry (Default)
- **Registry**: `ghcr.io`
- **Authentication**: Automatic via `GITHUB_TOKEN`
- **Visibility**: Private (repository-scoped)
- **Image naming**: `ghcr.io/[owner]/[repo]/radiocontestwinner`

### External Registry Setup (Optional)
To use external registries (Docker Hub, AWS ECR, etc.):

1. Update `REGISTRY` environment variable in workflow
2. Configure registry-specific secrets
3. Update login action in workflow:

```yaml
- name: Log in to Container Registry
  uses: docker/login-action@v3
  with:
    registry: ${{ env.REGISTRY }}
    username: ${{ secrets.DOCKER_REGISTRY_USERNAME }}
    password: ${{ secrets.DOCKER_REGISTRY_PASSWORD }}
```

## Security Configuration

### Vulnerability Scanning
- **Tool**: Trivy (built-in)
- **Format**: SARIF for GitHub Security tab integration
- **Scope**: Container images and dependencies
- **Frequency**: Every build

### Secret Management Best Practices
1. **Never commit secrets** to repository
2. **Use environment-specific secrets** for deployments
3. **Rotate secrets regularly** (recommended: quarterly)
4. **Limit secret scope** to minimum required permissions
5. **Monitor secret usage** in GitHub audit logs

## Troubleshooting

### Common Issues

#### 1. Docker Build Failures
```bash
# Check Dockerfile syntax
docker build -f repo/build/Dockerfile repo/

# Verify build context
ls -la repo/
```

#### 2. Registry Authentication Failures
- Verify `GITHUB_TOKEN` has `packages: write` permission
- Check repository visibility settings
- Ensure container registry is enabled for organization

#### 3. Test Failures
```bash
# Run tests locally
cd repo/
./scripts/coverage.sh
```

#### 4. Deployment Failures
- Verify environment secrets are configured
- Check deployment target connectivity
- Review deployment logs in Actions tab

### Debug Commands

#### Check workflow syntax:
```bash
# Install GitHub CLI
gh workflow list
gh workflow view ci-cd.yml
```

#### Manual workflow trigger:
```bash
gh workflow run ci-cd.yml
```

#### View workflow logs:
```bash
gh run list
gh run view [run-id] --log
```

## Monitoring and Notifications

### Workflow Status
- **Location**: Actions tab in GitHub repository
- **Notifications**: Configure in repository settings
- **Integration**: Slack/Teams webhooks (optional)

### Metrics to Monitor
- ✅ Build success rate
- ✅ Test coverage percentage
- ✅ Deployment frequency
- ✅ Security scan results
- ✅ Build duration trends

## Maintenance

### Regular Tasks
- **Weekly**: Review security scan results
- **Monthly**: Update base Docker images
- **Quarterly**: Rotate deployment secrets
- **Annually**: Review and update workflow configuration

### Updates
When updating the workflow:
1. Test changes in feature branch
2. Review workflow differences
3. Update this documentation
4. Notify team of changes

## Support

For issues with this CI/CD pipeline:
1. Check troubleshooting section above
2. Review GitHub Actions documentation
3. Check repository Issues tab
4. Contact DevOps team