# Tester Container Image

The Tester container image provides a test execution environment for the Arbiter Pipeline Infrastructure. It includes Node.js and Python testing frameworks and the `aphex-test` script for running automated tests.

## Base Image

- `node:20-alpine`

## Installed Tools

- **Node.js 20.x** and npm
- **Python 3.11** and pip
- **Jest** - JavaScript testing framework
- **Mocha** - JavaScript test framework
- **Pytest** - Python testing framework
- **AWS CLI v2** - For tests that interact with AWS services
- **Bash** - Shell for script execution

## Execution Script

### aphex-test

Executes test commands, captures output and exit codes, and reports pass/fail status.

**Usage:**
```bash
aphex-test <test-commands>
```

**Arguments:**
- `test-commands`: Test commands to execute (can be multiple commands separated by semicolons or newlines)

**Environment Variables:**
- `AWS_REGION`: AWS region (optional, for tests that interact with AWS)
- `AWS_ROLE_ARN`: IAM role to assume via IRSA (optional)

**Output:**
Structured JSON output with test results:
```json
{
  "success": true,
  "data": {
    "message": "Tests passed",
    "test_results": {
      "status": "pass",
      "exit_code": 0,
      "stdout": "...",
      "stderr": "...",
      "commands_executed": ["npm test", "pytest"]
    }
  },
  "timestamp": "2024-01-15T10:30:00.000000"
}
```

**Exit Codes:**
- `0`: Tests passed
- `1`: Invalid arguments
- `4`: Test execution error
- Other: Exit code from failed test command

## Building the Image

```bash
docker build -t arbiter/tester:latest .
```

## Running the Container

```bash
# Run Jest tests
docker run --rm \
  -v $(pwd):/workspace \
  arbiter/tester:latest \
  "npm test"

# Run Pytest tests
docker run --rm \
  -v $(pwd):/workspace \
  arbiter/tester:latest \
  "pytest tests/"

# Run multiple test commands
docker run --rm \
  -v $(pwd):/workspace \
  arbiter/tester:latest \
  "npm test; pytest tests/"
```

## Integration with Argo Workflows

Example WorkflowTemplate step:

```yaml
- name: test
  container:
    image: public.ecr.aws/arbiter/tester:latest
    command: ["/usr/local/bin/aphex-test"]
    args: ["npm test; pytest tests/"]
    volumeMounts:
      - name: workspace
        mountPath: /workspace
    env:
      - name: AWS_REGION
        value: "us-east-1"
```

## Features

- **Multi-framework support**: Supports Jest, Mocha, and Pytest out of the box
- **Structured output**: JSON output for easy parsing by workflow engines
- **Exit code capture**: Properly captures and reports test exit codes
- **Output capture**: Captures both stdout and stderr from test execution
- **CloudWatch integration**: Logs are automatically sent to CloudWatch when running in EKS
- **IRSA support**: Can assume IAM roles for tests that interact with AWS services

## Requirements Validation

This container image satisfies the following requirements:

- **Requirement 3.3**: Tester container image with test execution frameworks
- **Requirement 3.5**: Container images published to public container registry
- **Requirement 6.1**: aphex-test script accepts test commands as parameters
- **Requirement 6.2**: Script runs provided test commands
- **Requirement 6.3**: Script captures stdout and stderr output
- **Requirement 6.4**: Script captures exit code
- **Requirement 6.5**: Script outputs pass/fail status based on exit code
