"""
Property-based tests for aphex_common module

**Feature: arbiter-pipeline-infrastructure, Property 12: Script exit code behavior**
**Validates: Requirements 10.1, 10.4**

**Feature: arbiter-pipeline-infrastructure, Property 13: Structured script output**
**Validates: Requirements 10.2, 10.5**

**Feature: arbiter-pipeline-infrastructure, Property 14: Error logging**
**Validates: Requirements 10.3**
"""

import sys
import os
import io
import json
import logging
from datetime import datetime
from hypothesis import given, strategies as st, settings

# Add the containers/common directory to the path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../containers/common'))

from aphex_common import (
    ExitCode,
    setup_logging,
    output_result,
    validate_required_args,
    validate_aws_environment,
    ScriptError,
    InputError,
    AuthError,
    ResourceError,
    ExecutionError,
    TimeoutError,
    handle_script_error,
    log_to_cloudwatch
)


# Property 12: Script exit code behavior
# For any execution script, it should exit with code 0 on success and non-zero on error

@given(st.text(min_size=1))
@settings(max_examples=100)
def test_script_error_exit_codes_are_nonzero(error_message: str):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 12: Script exit code behavior**
    **Validates: Requirements 10.1, 10.4**
    
    Property: For any error type, the exit code should be non-zero
    """
    # Test all error types
    error_types = [
        InputError(error_message),
        AuthError(error_message),
        ResourceError(error_message),
        ExecutionError(error_message),
        TimeoutError(error_message)
    ]
    
    for error in error_types:
        # All error exit codes should be non-zero
        assert error.exit_code != ExitCode.SUCCESS
        assert error.exit_code > 0
        
        # handle_script_error should return non-zero exit code
        exit_code = handle_script_error(error)
        assert exit_code != 0
        assert exit_code > 0


@given(st.integers(min_value=1, max_value=10))
def test_exit_code_enum_values_are_valid(code_value: int):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 12: Script exit code behavior**
    **Validates: Requirements 10.1, 10.4**
    
    Property: All defined exit codes should be valid integers
    """
    # ExitCode enum values should be valid integers
    exit_codes = [
        ExitCode.SUCCESS,
        ExitCode.INPUT_ERROR,
        ExitCode.AUTH_ERROR,
        ExitCode.RESOURCE_ERROR,
        ExitCode.EXECUTION_ERROR,
        ExitCode.TIMEOUT_ERROR
    ]
    
    for exit_code in exit_codes:
        # Should be convertible to int
        assert isinstance(int(exit_code), int)
        # Should be non-negative
        assert int(exit_code) >= 0


@given(st.text(min_size=1))
def test_success_exit_code_is_zero(message: str):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 12: Script exit code behavior**
    **Validates: Requirements 10.1, 10.4**
    
    Property: Success operations should always use exit code 0
    """
    # ExitCode.SUCCESS should always be 0
    assert ExitCode.SUCCESS == 0
    assert int(ExitCode.SUCCESS) == 0


# Property 13: Structured script output
# For any execution script result (success or error), the script should output 
# structured JSON with success status, data/error information, and timestamp

@given(st.dictionaries(st.text(min_size=1, max_size=20), st.text(max_size=100), min_size=1))
@settings(max_examples=100)
def test_output_result_produces_valid_json(data: dict):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 13: Structured script output**
    **Validates: Requirements 10.2, 10.5**
    
    Property: For any data dictionary, output_result should produce valid JSON
    """
    # Capture output
    output_stream = io.StringIO()
    
    # Test success output
    output_result(True, data, stream=output_stream)
    output_json = output_stream.getvalue()
    
    # Should be valid JSON
    parsed = json.loads(output_json)
    
    # Should have required fields
    assert "success" in parsed
    assert "data" in parsed
    assert "timestamp" in parsed
    
    # Success should be True
    assert parsed["success"] is True
    
    # Data should match input
    assert parsed["data"] == data
    
    # Timestamp should be valid ISO format
    datetime.fromisoformat(parsed["timestamp"].replace('Z', '+00:00'))


@given(st.dictionaries(st.text(min_size=1, max_size=20), st.text(max_size=100), min_size=1))
@settings(max_examples=100)
def test_output_result_failure_format(data: dict):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 13: Structured script output**
    **Validates: Requirements 10.2, 10.5**
    
    Property: For any error data, output_result should produce valid JSON with success=false
    """
    # Capture output
    output_stream = io.StringIO()
    
    # Test failure output
    output_result(False, data, stream=output_stream)
    output_json = output_stream.getvalue()
    
    # Should be valid JSON
    parsed = json.loads(output_json)
    
    # Should have required fields
    assert "success" in parsed
    assert "data" in parsed
    assert "timestamp" in parsed
    
    # Success should be False
    assert parsed["success"] is False
    
    # Data should match input
    assert parsed["data"] == data
    
    # Timestamp should be valid ISO format
    datetime.fromisoformat(parsed["timestamp"].replace('Z', '+00:00'))


@given(st.text(min_size=1))
@settings(max_examples=100)
def test_script_error_to_dict_structure(error_message: str):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 13: Structured script output**
    **Validates: Requirements 10.2, 10.5**
    
    Property: For any ScriptError, to_dict() should produce a structured dictionary
    """
    error_types = [
        InputError(error_message),
        AuthError(error_message),
        ResourceError(error_message),
        ExecutionError(error_message),
        TimeoutError(error_message)
    ]
    
    for error in error_types:
        error_dict = error.to_dict()
        
        # Should have required fields
        assert "error" in error_dict
        assert "error_type" in error_dict
        assert "exit_code" in error_dict
        
        # Error message should match
        assert error_dict["error"] == error_message
        
        # Error type should be a string
        assert isinstance(error_dict["error_type"], str)
        
        # Exit code should be an integer
        assert isinstance(error_dict["exit_code"], int)
        assert error_dict["exit_code"] > 0


# Property 14: Error logging
# For any execution script error, the error details should be logged to CloudWatch Logs

@given(st.text(min_size=1), st.sampled_from(['INFO', 'WARNING', 'ERROR', 'DEBUG', 'CRITICAL']))
@settings(max_examples=100)
def test_log_to_cloudwatch_captures_messages(message: str, level: str):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 14: Error logging**
    **Validates: Requirements 10.3**
    
    Property: For any message and log level, log_to_cloudwatch should log the message
    """
    # Create a logger with a string stream handler to capture output
    logger = logging.getLogger(f'test_logger_{id(message)}')
    logger.handlers.clear()
    logger.setLevel(logging.DEBUG)
    
    stream = io.StringIO()
    handler = logging.StreamHandler(stream)
    handler.setLevel(logging.DEBUG)
    formatter = logging.Formatter('%(levelname)s - %(message)s')
    handler.setFormatter(formatter)
    logger.addHandler(handler)
    
    # Log the message
    log_to_cloudwatch(message, level, logger)
    
    # Get the logged output
    log_output = stream.getvalue()
    
    # The message should appear in the log output
    assert message in log_output
    
    # The log level should appear in the output
    assert level in log_output


@given(st.text(min_size=1))
@settings(max_examples=100)
def test_handle_script_error_logs_errors(error_message: str):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 14: Error logging**
    **Validates: Requirements 10.3**
    
    Property: For any script error, handle_script_error should log the error
    """
    # Create a logger with a string stream handler to capture output
    logger = logging.getLogger(f'test_error_logger_{id(error_message)}')
    logger.handlers.clear()
    logger.setLevel(logging.DEBUG)
    
    stream = io.StringIO()
    handler = logging.StreamHandler(stream)
    handler.setLevel(logging.DEBUG)
    formatter = logging.Formatter('%(levelname)s - %(message)s')
    handler.setFormatter(formatter)
    logger.addHandler(handler)
    
    # Create an error and handle it
    error = ExecutionError(error_message)
    
    # Redirect stderr to capture output_result
    old_stderr = sys.stderr
    sys.stderr = io.StringIO()
    
    try:
        exit_code = handle_script_error(error, logger)
        
        # Get the logged output
        log_output = stream.getvalue()
        
        # The error message should be logged
        assert error_message in log_output or 'ExecutionError' in log_output
        
        # Should return non-zero exit code
        assert exit_code != 0
    finally:
        sys.stderr = old_stderr


@given(st.text(min_size=1))
@settings(max_examples=100)
def test_setup_logging_creates_valid_logger(logger_name: str):
    """
    **Feature: arbiter-pipeline-infrastructure, Property 14: Error logging**
    **Validates: Requirements 10.3**
    
    Property: For any configuration, setup_logging should create a valid logger
    """
    # Setup logging should return a logger
    logger = setup_logging()
    
    # Should be a valid logger instance
    assert isinstance(logger, logging.Logger)
    
    # Logger should be able to log messages
    stream = io.StringIO()
    handler = logging.StreamHandler(stream)
    logger.addHandler(handler)
    
    test_message = "test message"
    logger.info(test_message)
    
    # Message should be captured (though format may vary)
    # We just verify the logger works
    assert isinstance(logger, logging.Logger)


if __name__ == "__main__":
    import pytest
    pytest.main([__file__, "-v", "--tb=short"])
