#!/usr/bin/env python3
"""
Example script demonstrating the use of aphex_common utilities

This is a reference implementation showing how to use the common utilities
in an aphex-* execution script.
"""

import sys
from aphex_common import (
    setup_logging,
    output_result,
    validate_required_args,
    validate_aws_environment,
    ExitCode,
    InputError,
    ExecutionError,
    handle_script_error,
    log_to_cloudwatch
)


def process_data(input_value: str) -> str:
    """Example processing function"""
    if not input_value:
        raise InputError("Input value cannot be empty")
    
    # Simulate some processing
    result = input_value.upper()
    return result


def main():
    """Main execution function"""
    logger = setup_logging()
    
    # Validate arguments
    usage = "Usage: example_script.py <input-value>"
    if not validate_required_args(sys.argv, 2, usage):
        sys.exit(ExitCode.INPUT_ERROR)
    
    input_value = sys.argv[1]
    
    # Validate AWS environment (optional for this example)
    is_valid, region, error = validate_aws_environment()
    if not is_valid:
        logger.warning(f"AWS environment not configured: {error}")
        # For this example, we'll continue anyway
    else:
        logger.info(f"AWS region: {region}")
    
    try:
        # Log operation start
        log_to_cloudwatch(f"Processing input: {input_value}", "INFO", logger)
        
        # Process the data
        result = process_data(input_value)
        
        # Log success
        log_to_cloudwatch("Processing completed successfully", "INFO", logger)
        
        # Output success result
        output_result(True, {
            "message": "Processing completed",
            "input": input_value,
            "result": result
        })
        
        sys.exit(ExitCode.SUCCESS)
        
    except Exception as e:
        # Handle any errors with consistent error handling
        exit_code = handle_script_error(e, logger)
        sys.exit(exit_code)


if __name__ == "__main__":
    main()
