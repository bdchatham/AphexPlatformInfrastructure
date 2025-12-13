# Builder Container Image

The Builder container image provides a complete build environment for CDK applications with Node.js, Python, Git, and AWS CLI.

## Base Image

- `node:20-alpine`

## Installed Tools

- Node.js 20.x and npm
- Python 3.11 and pip
- Git 2.x
- AWS CLI v2
- Build essentials (gcc, make, g++)

## Execution Script

### aphex-build

Clones a repository, executes build commands, packages artifacts, and uploads to S3.

**Usage:**
```bash
aphex-build <repo-url> <commit-sha> <build-commands> <artifact-bucket>
```

**Arguments:**
- `repo-url`: Git repository URL to clone
- `commit-sha`: Specific commit SHA to checkout
- `build-commands`: Build commands to execute (semicolon or newline separated)
- `artifact-bucket`: S3 bucket name for artifact upload

**Environment Variables:**
- `AWS_REGION`: AWS region for S3 operations (required)
- `AWS_ROLE_ARN`: IAM role to assume via IRSA (optional)

**Output:**
Structured JSON to stdout on success, stderr on failure:
```json
{
  "success": true,
  "data": {
    "message": "Build completed successfully",
    "artifact_s3_path": "s3://bucket/artifacts/build-abc12345-20231201-120000.tar.gz",
    "artifact_size": 1234567,
    "commit_sha": "abc123...",
    "timestamp": "20231201-120000"
  },
  "timestamp": "2023-12-01T12:00:00.000000"
}
```

**Exit Codes:**
- `0`: Success
- `1`: Invalid arguments
- `2`: Authentication error (missing AWS_REGION)
- `3`: Resource error (clone/S3 failure)
- `4`: Execution error (build failure)

## Building the Image

```bash
docker build -t arbiter/builder:latest containers/builder/
```

## Running the Container

```bash
docker run --rm \
  -e AWS_REGION=us-east-1 \
  -e AWS_ACCESS_KEY_ID=... \
  -e AWS_SECRET_ACCESS_KEY=... \
  arbiter/builder:latest \
  https://github.com/example/repo.git \
  abc123def456 \
  "npm install; npm run build" \
  my-artifact-bucket
```

## Example with IRSA (in Kubernetes)

```yaml
apiVersion: v1
kind: Pod
metadata:
  name: builder-pod
spec:
  serviceAccountName: builder-sa
  containers:
  - name: builder
    image: public.ecr.aws/arbiter/builder:latest
    args:
    - "https://github.com/example/repo.git"
    - "abc123def456"
    - "npm install; npm run build"
    - "my-artifact-bucket"
    env:
    - name: AWS_REGION
      value: "us-east-1"
```

## Artifact Structure

Artifacts are packaged as gzipped tarballs with the following naming convention:
```
build-{commit-sha-short}-{timestamp}.tar.gz
```

Example: `build-abc12345-20231201-120000.tar.gz`

The artifact includes the entire repository (excluding `.git` directory) after build commands have been executed.

## S3 Metadata

Uploaded artifacts include the following S3 metadata:
- `commit-sha`: Full commit SHA
- `timestamp`: Build timestamp in format YYYYMMDD-HHMMSS
