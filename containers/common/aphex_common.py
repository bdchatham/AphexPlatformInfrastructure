#!/usr/bin/env python3
"""
aphex_common: Common utilities for Arbiter Pipeline Infrastructure execution scripts

This module provides shared functionality for structured output, logging,
AWS credential handling, and error management across all aphex-* scripts.
"""

import sys
import json
import logging
import os
from datetime import datetime, timezone
from typing import Dict, Any, Optional
from enum import IntEnum


class ExitCode(IntEnum):
    """Standard exit codes for aphex scripts"""
    SUCCESS = 0
    INPUT_ERROR = 1
    AUTH_ERROR = 2
    RESOURCE_ERROR = 3
    EXECUTION_ERROR = 4
    TIMEOUT_ERROR = 5


def setup_logging(level: int = logging.INFO) -> logging.Logger:
    """
    Configure logging to output to stdout/stderr for CloudWatch
    
    Args:
        level: Logging level (default: logging.INFO)
    
    Returns:
        Configured logger instance
    """
    logging.basicConfig(
        level=level,
        format='%(asctime)s - %(name)s - %(levelname)s - %(message)s',
        stream=sys.stdout
    )
    
    logger = logging.getLogger('aphex')
    return logger


def output_result(success: bool, data: Dict[str, Any], stream: Optional[Any] = None):
    """
    Output structured JSON result to stdout (success) or stderr (failure)
    
    Args:
        success: Whether the operation succeeded
        data: Dictionary containing result data or error information
        stream: Optional output stream (defaults to stdout for success, stderr for failure)
    """
    result = {
        "success": success,
        "data": data,
        "timestamp": datetime.now(timezone.utc).isoformat()
    }
    
    output = json.dumps(result, indent=2)
    
    if stream is None:
        stream = sys.stdout if success else sys.stderr
    
    print(output, file=stream)


def validate_required_args(args: list, expected_count: int, usage_message: str) -> bool:
    """
    Validate that the correct number of command-line arguments were provided
    
    Args:
        args: sys.argv list
        expected_count: Expected number of arguments (including script name)
        usage_message: Usage message to display on error
    
    Returns:
        True if validation passes, False otherwise (and outputs error)
    """
    if len(args) != expected_count:
        logging.error(f"Invalid number of arguments: expected {expected_count}, got {len(args)}")
        output_result(False, {"error": usage_message})
        return False
    return True


def validate_aws_environment() -> tuple[bool, Optional[str], Optional[str]]:
    """
    Validate that required AWS environment variables are set
    
    Returns:
        Tuple of (is_valid, aws_region, error_message)
    """
    aws_region = os.environ.get('AWS_REGION')
    
    if not aws_region:
        error_msg = "AWS_REGION environment variable required"
        logging.error(error_msg)
        return False, None, error_msg
    
    logging.info(f"AWS environment validated: region={aws_region}")
    return True, aws_region, None


def validate_aws_account() -> tuple[bool, Optional[str], Optional[str]]:
    """
    Validate that AWS_ACCOUNT environment variable is set
    
    Returns:
        Tuple of (is_valid, aws_account, error_message)
    """
    aws_account = os.environ.get('AWS_ACCOUNT')
    
    if not aws_account:
        error_msg = "AWS_ACCOUNT environment variable required"
        logging.error(error_msg)
        return False, None, error_msg
    
    logging.info(f"AWS account validated: {aws_account}")
    return True, aws_account, None


def get_irsa_role() -> Optional[str]:
    """
    Get the IAM role ARN for IRSA (IAM Roles for Service Accounts)
    
    Returns:
        IAM role ARN if set, None otherwise
    """
    role_arn = os.environ.get('AWS_ROLE_ARN')
    
    if role_arn:
        logging.info(f"IRSA role detected: {role_arn}")
    else:
        logging.debug("No IRSA role configured (AWS_ROLE_ARN not set)")
    
    return role_arn


def verify_irsa_credentials() -> bool:
    """
    Verify that IRSA credentials are available and working
    
    This checks that:
    1. AWS_ROLE_ARN is set (indicating IRSA is configured)
    2. AWS_WEB_IDENTITY_TOKEN_FILE is set (the token file for IRSA)
    3. The token file exists and is readable
    
    Returns:
        True if IRSA credentials are properly configured, False otherwise
    """
    logger = logging.getLogger('aphex')
    
    # Check for IRSA role ARN
    role_arn = os.environ.get('AWS_ROLE_ARN')
    if not role_arn:
        logger.debug("AWS_ROLE_ARN not set - IRSA not configured")
        return False
    
    # Check for web identity token file
    token_file = os.environ.get('AWS_WEB_IDENTITY_TOKEN_FILE')
    if not token_file:
        logger.warning("AWS_ROLE_ARN is set but AWS_WEB_IDENTITY_TOKEN_FILE is not - IRSA misconfigured")
        return False
    
    # Verify token file exists
    if not os.path.exists(token_file):
        logger.error(f"IRSA token file does not exist: {token_file}")
        return False
    
    # Verify token file is readable
    if not os.access(token_file, os.R_OK):
        logger.error(f"IRSA token file is not readable: {token_file}")
        return False
    
    logger.info(f"IRSA credentials verified: role={role_arn}, token_file={token_file}")
    return True


def ensure_no_long_lived_credentials() -> tuple[bool, Optional[str]]:
    """
    Ensure that no long-lived AWS credentials are being used
    
    This verifies that:
    1. AWS_ACCESS_KEY_ID is not set (would indicate long-lived credentials)
    2. AWS_SECRET_ACCESS_KEY is not set (would indicate long-lived credentials)
    3. IRSA is properly configured instead
    
    Returns:
        Tuple of (is_valid, error_message)
    """
    logger = logging.getLogger('aphex')
    
    # Check for long-lived credentials
    if os.environ.get('AWS_ACCESS_KEY_ID') or os.environ.get('AWS_SECRET_ACCESS_KEY'):
        error_msg = "Long-lived AWS credentials detected (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY). Use IRSA instead."
        logger.error(error_msg)
        return False, error_msg
    
    # Verify IRSA is configured
    if not verify_irsa_credentials():
        error_msg = "IRSA credentials not properly configured. Ensure AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE are set."
        logger.error(error_msg)
        return False, error_msg
    
    logger.info("Credential validation passed: using IRSA, no long-lived credentials detected")
    return True, None


class ScriptError(Exception):
    """
    Base exception class for aphex script errors
    
    Attributes:
        message: Error message
        exit_code: Suggested exit code for this error
        error_type: Type of error for structured output
    """
    
    def __init__(self, message: str, exit_code: ExitCode = ExitCode.EXECUTION_ERROR):
        self.message = message
        self.exit_code = exit_code
        self.error_type = self.__class__.__name__
        super().__init__(self.message)
    
    def to_dict(self) -> Dict[str, Any]:
        """Convert error to dictionary for structured output"""
        return {
            "error": self.message,
            "error_type": self.error_type,
            "exit_code": int(self.exit_code)
        }


class InputError(ScriptError):
    """Error related to invalid input parameters"""
    
    def __init__(self, message: str):
        super().__init__(message, ExitCode.INPUT_ERROR)


class AuthError(ScriptError):
    """Error related to authentication or authorization"""
    
    def __init__(self, message: str):
        super().__init__(message, ExitCode.AUTH_ERROR)


class ResourceError(ScriptError):
    """Error related to resource access (S3, Git, etc.)"""
    
    def __init__(self, message: str):
        super().__init__(message, ExitCode.RESOURCE_ERROR)


class ExecutionError(ScriptError):
    """Error related to command execution"""
    
    def __init__(self, message: str):
        super().__init__(message, ExitCode.EXECUTION_ERROR)


class TimeoutError(ScriptError):
    """Error related to operation timeout"""
    
    def __init__(self, message: str):
        super().__init__(message, ExitCode.TIMEOUT_ERROR)


def handle_script_error(error: Exception, logger: Optional[logging.Logger] = None) -> int:
    """
    Handle script errors with consistent logging and output
    
    Args:
        error: Exception that occurred
        logger: Optional logger instance
    
    Returns:
        Exit code to use
    """
    if logger is None:
        logger = logging.getLogger('aphex')
    
    if isinstance(error, ScriptError):
        logger.error(f"{error.error_type}: {error.message}", exc_info=True)
        output_result(False, error.to_dict())
        return int(error.exit_code)
    else:
        # Generic exception
        logger.error(f"Unexpected error: {str(error)}", exc_info=True)
        output_result(False, {
            "error": str(error),
            "error_type": type(error).__name__
        })
        return int(ExitCode.EXECUTION_ERROR)


def log_to_cloudwatch(message: str, level: str = 'INFO', logger: Optional[logging.Logger] = None):
    """
    Log a message that will be captured by CloudWatch Logs
    
    Args:
        message: Message to log
        level: Log level (INFO, WARNING, ERROR, DEBUG)
        logger: Optional logger instance
    """
    if logger is None:
        logger = logging.getLogger('aphex')
    
    level_map = {
        'DEBUG': logging.DEBUG,
        'INFO': logging.INFO,
        'WARNING': logging.WARNING,
        'ERROR': logging.ERROR,
        'CRITICAL': logging.CRITICAL
    }
    
    log_level = level_map.get(level.upper(), logging.INFO)
    logger.log(log_level, message)
