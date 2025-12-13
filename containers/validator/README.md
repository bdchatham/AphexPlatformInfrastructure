# Validator Container

This container provides configuration validation capabilities for the Arbiter Pipeline Infrastructure.

## Purpose

The Validator container validates pipeline configurations, AWS credentials, and CDK context values before deployment to ensure all prerequisites are met.

## Base Image

- `python:3.11-slim`

## Installed Tools

- Python 3.11
- AWS CLI v2
- jsonschema (Python library)
- PyYAML (Python library)

## Execution Script

### aphex-validate

Validates configuration files and prerequisites.

**Usage:**
```bash
aphex-validate <config-path>
```

**Arguments:**
- `config-path`: Path to the configuration file (JSON or YAML format)

**Environment Variables:**
- `AWS_REGION`: AWS region (optional)
- `AWS_ROLE_ARN`: IAM role to assume via IRSA (optional)

**Validation Sequence:**
1. **JSON Schema Validation**: Validates the configuration structure against a predefined schema
2. **AWS Credentials Validation**: Verifies that AWS credentials are available and valid
3. **CDK Context Validation**: Checks that required CDK context values are present

**Output:**
- Structured JSON output with validation results
- Exit code 0 on success (all validations pass)
- Exit code 1 on failure (any validation fails)

**Example Configuration:**
```json
{
  "pipeline": {
    "name": "my-pipeline",
    "repository": "https://github.com/org/repo.git",
    "branch": "main"
  },
  "build": {
    "commands": ["npm install", "npm run build"],
    "artifactBucket": "my-artifacts-bucket"
  },
  "deploy": {
    "stacks": ["MyStack"],
    "environment": "production"
  },
  "test": {
    "commands": ["npm test"]
  },
  "cdk": {
    "context": {
      "vpc-id": "vpc-12345"
    },
    "requiredContext": ["vpc-id"]
  }
}
```

## Building the Image

```bash
docker build -t arbiter/validator:latest .
```

## Running the Container

```bash
docker run -v $(pwd)/config.json:/workspace/config.json \
  -e AWS_REGION=us-east-1 \
  arbiter/validator:latest /workspace/config.json
```

## Integration with Argo Workflows

This container is designed to be used as a validation step in Argo Workflows:

```yaml
- name: validate
  container:
    image: public.ecr.aws/arbiter/validator:latest
    args:
      - /workspace/pipeline-config.json
    volumeMounts:
      - name: config
        mountPath: /workspace
```

## Requirements

Validates requirements:
- 3.4: Validator container image with configuration validation tools
- 3.5: Container images published to public registry
- 7.1: Accept configuration file path as parameter
- 7.2: Validate configuration against JSON schema
- 7.3: Validate AWS credentials are available
- 7.4: Validate required CDK context values
- 7.5: Output success status when all validations pass
