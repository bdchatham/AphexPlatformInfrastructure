# Aphex Common Utilities

This module provides shared functionality for all Arbiter Pipeline Infrastructure execution scripts (`aphex-*`).

## Features

### 1. Structured Output Formatting

All scripts output structured JSON to stdout (success) or stderr (failure):

```python
from aphex_common import output_result

# Success output
output_result(True, {
    "message": "Operation completed",
    "result": "some-value"
})

# Error output
output_result(False, {
    "error": "Something went wrong",
    "error_type": "ExecutionError"
})
```

Output format:
```json
{
  "success": true,
  "data": { ... },
  "timestamp": "2024-01-01T12:00:00+00:00"
}
```

### 2. CloudWatch Logging Setup

Configure logging to output to stdout/stderr for CloudWatch capture:

```python
from aphex_common import setup_logging

logger = setup_logging()
logger.info("Starting operation")
logger.error("Operation failed")
```

### 3. AWS Credential Handling via IRSA

Validate AWS environment variables and detect IRSA roles:

```python
from aphex_common import validate_aws_environment, validate_aws_account, get_irsa_role

# Validate AWS_REGION
is_valid, region, error = validate_aws_environment()
if not is_valid:
    output_result(False, {"error": error})
    sys.exit(ExitCode.AUTH_ERROR)

# Validate AWS_ACCOUNT (for deployment scripts)
is_valid, account, error = validate_aws_account()
if not is_valid:
    output_result(False, {"error": error})
    sys.exit(ExitCode.AUTH_ERROR)

# Get IRSA role ARN (optional)
role_arn = get_irsa_role()
```

### 4. Error Handling and Exit Codes

Consistent error handling with standard exit codes:

```python
from aphex_common import (
    ExitCode,
    InputError,
    AuthError,
    ResourceError,
    ExecutionError,
    TimeoutError,
    handle_script_error
)

try:
    # Script logic
    if invalid_input:
        raise InputError("Invalid parameter: repo-url")
    
    if auth_failed:
        raise AuthError("AWS credentials not available")
    
    if resource_missing:
        raise ResourceError("S3 bucket not found")
    
    if command_failed:
        raise ExecutionError("Build command failed")
    
    if timeout:
        raise TimeoutError("Operation timed out")
    
except Exception as e:
    exit_code = handle_script_error(e, logger)
    sys.exit(exit_code)
```

### Exit Code Reference

| Exit Code | Constant | Description |
|-----------|----------|-------------|
| 0 | `ExitCode.SUCCESS` | Operation completed successfully |
| 1 | `ExitCode.INPUT_ERROR` | Invalid parameters or arguments |
| 2 | `ExitCode.AUTH_ERROR` | Authentication or authorization failure |
| 3 | `ExitCode.RESOURCE_ERROR` | Resource access failure (S3, Git, etc.) |
| 4 | `ExitCode.EXECUTION_ERROR` | Command execution failure |
| 5 | `ExitCode.TIMEOUT_ERROR` | Operation timeout |

### 5. Argument Validation

Validate command-line arguments:

```python
from aphex_common import validate_required_args

if not validate_required_args(sys.argv, 5, "Usage: script <arg1> <arg2> <arg3> <arg4>"):
    sys.exit(ExitCode.INPUT_ERROR)
```

### 6. CloudWatch Logging

Log messages that will be captured by CloudWatch:

```python
from aphex_common import log_to_cloudwatch

log_to_cloudwatch("Starting deployment", "INFO", logger)
log_to_cloudwatch("Deployment failed", "ERROR", logger)
```

## Complete Example

```python
#!/usr/bin/env python3
import sys
from aphex_common import (
    setup_logging,
    output_result,
    validate_required_args,
    validate_aws_environment,
    ExitCode,
    ExecutionError,
    handle_script_error
)

def main():
    logger = setup_logging()
    
    # Validate arguments
    if not validate_required_args(sys.argv, 3, "Usage: script <arg1> <arg2>"):
        sys.exit(ExitCode.INPUT_ERROR)
    
    arg1 = sys.argv[1]
    arg2 = sys.argv[2]
    
    # Validate AWS environment
    is_valid, region, error = validate_aws_environment()
    if not is_valid:
        output_result(False, {"error": error})
        sys.exit(ExitCode.AUTH_ERROR)
    
    try:
        # Script logic
        logger.info(f"Processing {arg1} and {arg2}")
        
        # ... do work ...
        
        output_result(True, {
            "message": "Operation completed",
            "result": "success"
        })
        sys.exit(ExitCode.SUCCESS)
        
    except Exception as e:
        exit_code = handle_script_error(e, logger)
        sys.exit(exit_code)

if __name__ == "__main__":
    main()
```

## Testing

Property-based tests verify the correctness properties:

- **Property 12**: Script exit code behavior - scripts exit with 0 on success, non-zero on error
- **Property 13**: Structured script output - all output is valid JSON with required fields
- **Property 14**: Error logging - all errors are logged to CloudWatch

Run tests:
```bash
pytest test/common/aphex_common_property_test.py -v
```

## Integration with Existing Scripts

To integrate this module into existing scripts:

1. Add import at the top:
```python
import sys
import os
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../common'))
from aphex_common import *
```

2. Replace existing `setup_logging()` and `output_result()` functions with imports

3. Replace exit codes with `ExitCode` enum values

4. Wrap main logic in try/except with `handle_script_error()`

5. Use error classes (`InputError`, `AuthError`, etc.) instead of generic exceptions
