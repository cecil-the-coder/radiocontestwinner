# CI/CD Workflow Troubleshooting Guide

## Quick Diagnosis

### Workflow Status Check
```bash
# Check current workflow status
gh workflow list
gh run list --limit 5

# View specific run details
gh run view [run-id] --log
```

## Common Issues and Solutions

### 1. Build Failures

#### Go Module Download Issues
**Symptoms:**
- `go mod download` fails in workflow
- Module not found errors

**Solutions:**
```bash
# Local testing
cd repo/
go mod verify
go mod tidy

# Check for invalid modules
go list -m all
```

**Workflow Fix:**
Ensure `go.mod` and `go.sum` are properly committed.

#### Missing System Dependencies
**Symptoms:**
- `bc: command not found` during coverage calculation
- Build tools missing in container

**Solution:**
The workflow installs required dependencies:
```yaml
- name: Install dependencies
  run: |
    sudo apt-get update
    sudo apt-get install -y ffmpeg build-essential pkg-config cmake bc
```

### 2. Test Failures

#### Coverage Requirements Not Met
**Symptoms:**
- Coverage script fails with "Coverage X% is below required 80%"
- Tests pass but build fails on coverage check

**Debug Commands:**
```bash
# Run coverage locally
cd repo/
./scripts/coverage.sh

# Check specific package coverage
go test -v -coverprofile=coverage.out ./...
go tool cover -func=coverage.out
```

**Solutions:**
1. **Add more tests** to reach 80% coverage
2. **Check coverage exemptions** in script for small codebases
3. **Review coverage calculation** in `scripts/coverage.sh`

#### Test Timeouts
**Symptoms:**
- Tests hang or timeout in CI
- Integration tests fail inconsistently

**Solutions:**
```yaml
# Add timeout to test steps
- name: Run tests with coverage enforcement
  timeout-minutes: 10
  working-directory: ./repo
  run: ./scripts/coverage.sh
```

### 3. Docker Build Issues

#### Build Context Problems
**Symptoms:**
- Files not found during Docker build
- COPY commands fail

**Debug Commands:**
```bash
# Check build context
cd repo/
docker build -t test-build -f build/Dockerfile .

# Verify files in context
ls -la build/
```

**Solution:**
Ensure Dockerfile is in correct location: `repo/build/Dockerfile`

#### Multi-stage Build Failures
**Symptoms:**
- Build stage succeeds but runtime stage fails
- Missing files in final image

**Debug Commands:**
```bash
# Build only specific stage
docker build --target builder -t test-builder -f build/Dockerfile .
docker build --target runtime -t test-runtime -f build/Dockerfile .

# Inspect image layers
docker history radiocontestwinner-test:latest
```

#### CGo Compilation Issues
**Symptoms:**
- `cgo: C compiler not found` errors
- Whisper.cpp binding failures

**Solution:**
Ensure build stage has required tools:
```dockerfile
RUN apt-get update && apt-get install -y \
    build-essential \
    pkg-config \
    cmake \
    git \
    bc
```

### 4. Container Registry Issues

#### Authentication Failures
**Symptoms:**
- `docker/login-action` fails
- Permission denied pushing to registry

**Debug Steps:**
1. **Check GITHUB_TOKEN permissions:**
   - Repository Settings → Actions → General
   - Ensure "Read and write permissions" enabled

2. **Verify registry access:**
```bash
# Test local login
echo $GITHUB_TOKEN | docker login ghcr.io -u $GITHUB_USERNAME --password-stdin
```

3. **Check package permissions:**
   - GitHub → Settings → Developer settings → Personal access tokens
   - Ensure `write:packages` scope

#### Image Push Failures
**Symptoms:**
- Build succeeds but push fails
- Registry quota exceeded

**Solutions:**
1. **Check storage quota** in GitHub organization settings
2. **Clean up old images:**
```bash
# List packages
gh api /user/packages?package_type=container

# Delete old versions
gh api -X DELETE /user/packages/container/radiocontestwinner/versions/[version-id]
```

### 5. Deployment Issues

#### Environment Configuration
**Symptoms:**
- Deployment jobs skip unexpectedly
- Environment secrets not available

**Debug Steps:**
1. **Check environment conditions:**
```yaml
# Verify branch conditions
if: github.ref == 'refs/heads/main'

# Check environment setup
environment: staging
```

2. **Verify environment secrets:**
   - Repository Settings → Environments
   - Check secret names match workflow variables

#### Missing Deployment Targets
**Symptoms:**
- Deployment succeeds but nothing happens
- No actual deployment occurs

**Solution:**
Implement actual deployment commands in workflow:
```yaml
- name: Deploy to staging
  run: |
    # Replace with actual deployment commands
    kubectl apply -f k8s/staging/
    helm upgrade radiocontestwinner ./helm-chart
```

### 6. Security Scanning Issues

#### Trivy Scanner Failures
**Symptoms:**
- Security scan job fails
- Vulnerability database update errors

**Solutions:**
1. **Update Trivy action version:**
```yaml
uses: aquasecurity/trivy-action@master
```

2. **Add fallback for scan failures:**
```yaml
- name: Run Trivy vulnerability scanner
  uses: aquasecurity/trivy-action@master
  continue-on-error: true  # Don't fail build on scan issues
```

#### SARIF Upload Issues
**Symptoms:**
- Security results don't appear in Security tab
- SARIF upload fails

**Solution:**
```yaml
- name: Upload Trivy scan results
  uses: github/codeql-action/upload-sarif@v3
  if: always()  # Upload even if scan partially fails
  with:
    sarif_file: 'trivy-results.sarif'
```

## Performance Optimization

### Build Speed Issues
**Symptoms:**
- Builds take too long
- Frequent cache misses

**Solutions:**
1. **Optimize Docker layer caching:**
```dockerfile
# Copy go.mod first for better caching
COPY go.mod go.sum ./
RUN go mod download
COPY . .
```

2. **Use GitHub Actions cache:**
```yaml
- name: Cache Go modules
  uses: actions/cache@v4
  with:
    path: |
      ~/.cache/go-build
      ~/go/pkg/mod
    key: ${{ runner.os }}-go-${{ hashFiles('**/go.sum') }}
```

3. **Enable Docker BuildKit:**
```yaml
env:
  DOCKER_BUILDKIT: 1
```

### Resource Constraints
**Symptoms:**
- Jobs fail with "out of memory" errors
- Timeouts in resource-intensive operations

**Solutions:**
1. **Increase job timeouts:**
```yaml
jobs:
  test-and-build:
    timeout-minutes: 30  # Increase from default 360 minutes
```

2. **Optimize test execution:**
```bash
# Run tests in parallel
go test -parallel 4 ./...

# Reduce test verbosity in CI
go test -v ./... > test-results.log 2>&1
```

## Monitoring and Alerts

### Setting Up Notifications
**Slack Integration:**
```yaml
- name: Notify Slack on failure
  if: failure()
  uses: 8398a7/action-slack@v3
  with:
    status: failure
    webhook_url: ${{ secrets.SLACK_WEBHOOK }}
```

**Email Notifications:**
Configure in Repository Settings → Notifications

### Key Metrics to Monitor
- ✅ Build success rate (target: >95%)
- ✅ Average build time (target: <10 minutes)
- ✅ Test coverage percentage (target: >80%)
- ✅ Security vulnerabilities (target: 0 high/critical)
- ✅ Deployment frequency (target: daily)

## Emergency Procedures

### Rollback Deployment
```bash
# Rollback using kubectl
kubectl rollout undo deployment/radiocontestwinner -n production

# Rollback using Helm
helm rollback radiocontestwinner 1 -n production
```

### Disable Workflow
```bash
# Disable workflow temporarily
gh workflow disable ci-cd.yml

# Re-enable workflow
gh workflow enable ci-cd.yml
```

### Emergency Hotfix Process
1. Create hotfix branch from main
2. Make minimal necessary changes
3. Ensure tests pass locally
4. Create PR with emergency label
5. Fast-track review and merge
6. Monitor deployment closely

## Maintenance Tasks

### Weekly
- [ ] Review failed workflows and identify patterns
- [ ] Check security scan results
- [ ] Monitor build performance metrics
- [ ] Update dependencies if needed

### Monthly
- [ ] Update GitHub Actions to latest versions
- [ ] Review and clean up old container images
- [ ] Assess and update security policies
- [ ] Performance optimization review

### Quarterly
- [ ] Full security audit of workflow
- [ ] Review and update deployment strategies
- [ ] Disaster recovery testing
- [ ] Update troubleshooting documentation

## Getting Help

### Internal Resources
1. Check this troubleshooting guide
2. Review workflow logs in GitHub Actions
3. Check repository Issues tab for similar problems

### External Resources
1. [GitHub Actions Documentation](https://docs.github.com/en/actions)
2. [Docker Build Documentation](https://docs.docker.com/engine/reference/builder/)
3. [Go Testing Documentation](https://golang.org/doc/tutorial/add-a-test)

### Contact Information
- **DevOps Team**: [devops@example.com](mailto:devops@example.com)
- **Security Team**: [security@example.com](mailto:security@example.com)
- **Emergency Escalation**: [oncall@example.com](mailto:oncall@example.com)

## Workflow Health Checklist

Before considering the CI/CD pipeline healthy, verify:

- [ ] ✅ All jobs pass consistently
- [ ] ✅ Build times are reasonable (<10 minutes)
- [ ] ✅ Test coverage meets requirements (>80%)
- [ ] ✅ Security scans show no critical vulnerabilities
- [ ] ✅ Deployments succeed to all environments
- [ ] ✅ Rollback procedures work correctly
- [ ] ✅ Monitoring and alerts are functional
- [ ] ✅ Documentation is up to date

---

*Last updated: 2025-09-19*
*Version: 1.0*