#!/usr/bin/env python3
"""
Property-based tests for IRSA credential provisioning

**Feature: arbiter-pipeline-infrastructure, Property 15: IRSA credential provisioning**
**Validates: Requirements 11.1, 11.2**

These tests verify that:
1. IRSA credentials are provided via environment variables (no long-lived credentials)
2. AWS_ROLE_ARN and AWS_WEB_IDENTITY_TOKEN_FILE are set
3. The token file exists and is readable
4. Long-lived credentials (AWS_ACCESS_KEY_ID/AWS_SECRET_ACCESS_KEY) are not used
"""

import os
import sys
import tempfile
import unittest
from unittest.mock import patch, MagicMock
from hypothesis import given, strategies as st, settings, assume, HealthCheck
from pathlib import Path

# Add common module to path
sys.path.insert(0, os.path.join(os.path.dirname(__file__), '../../containers/common'))
from aphex_common import (
    get_irsa_role,
    verify_irsa_credentials,
    ensure_no_long_lived_credentials
)


# Custom strategy for generating valid IAM role ARNs
@st.composite
def iam_role_arn(draw):
    """Generate a valid IAM role ARN"""
    account_id = draw(st.integers(min_value=100000000000, max_value=999999999999))
    role_name = draw(st.text(
        alphabet=st.characters(whitelist_categories=('Lu', 'Ll', 'Nd'), whitelist_characters='-_'),
        min_size=1,
        max_size=64
    ))
    return f"arn:aws:iam::{account_id}:role/{role_name}"


# Custom strategy for generating valid AWS access keys (alphanumeric only, no null bytes)
@st.composite
def aws_access_key(draw):
    """Generate a valid AWS access key ID"""
    return draw(st.text(
        alphabet=st.characters(whitelist_categories=('Lu', 'Nd')),
        min_size=16,
        max_size=32
    ))


# Custom strategy for generating valid AWS secret keys (alphanumeric + special chars, no null bytes)
@st.composite
def aws_secret_key(draw):
    """Generate a valid AWS secret access key"""
    return draw(st.text(
        alphabet=st.characters(whitelist_categories=('Lu', 'Ll', 'Nd'), whitelist_characters='+/='),
        min_size=32,
        max_size=64
    ))


class TestIRSACredentialProvisioning(unittest.TestCase):
    """
    Property-based tests for IRSA credential provisioning
    
    Property 15: For any workflow pod execution, AWS credentials should be provided
    via IRSA without long-lived access keys
    """
    
    def setUp(self):
        """Save original environment"""
        self.original_env = os.environ.copy()
    
    def tearDown(self):
        """Restore original environment"""
        os.environ.clear()
        os.environ.update(self.original_env)
    
    @given(role_arn=iam_role_arn())
    @settings(max_examples=100)
    def test_property_irsa_role_detection(self, role_arn):
        """
        Property: For any valid IAM role ARN, get_irsa_role should detect it
        when AWS_ROLE_ARN is set
        """
        # Arrange: Set AWS_ROLE_ARN environment variable
        os.environ['AWS_ROLE_ARN'] = role_arn
        
        # Act: Get IRSA role
        detected_role = get_irsa_role()
        
        # Assert: The detected role should match the set role
        self.assertEqual(detected_role, role_arn)
    
    def test_property_no_irsa_role_returns_none(self):
        """
        Property: When AWS_ROLE_ARN is not set, get_irsa_role should return None
        """
        # Arrange: Ensure AWS_ROLE_ARN is not set
        os.environ.pop('AWS_ROLE_ARN', None)
        
        # Act: Get IRSA role
        detected_role = get_irsa_role()
        
        # Assert: Should return None
        self.assertIsNone(detected_role)
    
    @given(role_arn=iam_role_arn())
    @settings(max_examples=100)
    def test_property_irsa_credentials_with_valid_token_file(self, role_arn):
        """
        Property: For any valid role ARN and existing token file,
        verify_irsa_credentials should return True
        """
        # Arrange: Create a temporary token file
        with tempfile.NamedTemporaryFile(mode='w', delete=False) as token_file:
            token_file.write('mock-token-content')
            token_file_path = token_file.name
        
        try:
            os.environ['AWS_ROLE_ARN'] = role_arn
            os.environ['AWS_WEB_IDENTITY_TOKEN_FILE'] = token_file_path
            
            # Act: Verify IRSA credentials
            result = verify_irsa_credentials()
            
            # Assert: Should return True
            self.assertTrue(result)
        finally:
            # Cleanup
            os.unlink(token_file_path)
    
    @given(role_arn=iam_role_arn())
    @settings(max_examples=100)
    def test_property_irsa_credentials_missing_token_file(self, role_arn):
        """
        Property: For any role ARN with non-existent token file,
        verify_irsa_credentials should return False
        """
        # Arrange: Set role ARN but point to non-existent file
        os.environ['AWS_ROLE_ARN'] = role_arn
        os.environ['AWS_WEB_IDENTITY_TOKEN_FILE'] = '/nonexistent/token/file'
        
        # Act: Verify IRSA credentials
        result = verify_irsa_credentials()
        
        # Assert: Should return False
        self.assertFalse(result)
    
    @given(
        access_key=aws_access_key(),
        secret_key=aws_secret_key()
    )
    @settings(max_examples=100)
    def test_property_reject_long_lived_credentials(self, access_key, secret_key):
        """
        Property: For any access key and secret key, ensure_no_long_lived_credentials
        should reject them and return False
        
        This validates that long-lived credentials are never accepted
        """
        # Arrange: Set long-lived credentials
        os.environ['AWS_ACCESS_KEY_ID'] = access_key
        os.environ['AWS_SECRET_ACCESS_KEY'] = secret_key
        os.environ.pop('AWS_ROLE_ARN', None)
        os.environ.pop('AWS_WEB_IDENTITY_TOKEN_FILE', None)
        
        # Act: Check for long-lived credentials
        is_valid, error_msg = ensure_no_long_lived_credentials()
        
        # Assert: Should reject long-lived credentials
        self.assertFalse(is_valid)
        self.assertIsNotNone(error_msg)
        self.assertIn('Long-lived', error_msg)
    
    @given(role_arn=iam_role_arn())
    @settings(max_examples=100)
    def test_property_accept_irsa_credentials(self, role_arn):
        """
        Property: For any valid IRSA configuration (role ARN + token file),
        ensure_no_long_lived_credentials should accept it and return True
        
        This validates that IRSA credentials are properly accepted
        """
        # Arrange: Create a temporary token file and set IRSA environment
        with tempfile.NamedTemporaryFile(mode='w', delete=False) as token_file:
            token_file.write('mock-token-content')
            token_file_path = token_file.name
        
        try:
            # Ensure no long-lived credentials
            os.environ.pop('AWS_ACCESS_KEY_ID', None)
            os.environ.pop('AWS_SECRET_ACCESS_KEY', None)
            
            # Set IRSA credentials
            os.environ['AWS_ROLE_ARN'] = role_arn
            os.environ['AWS_WEB_IDENTITY_TOKEN_FILE'] = token_file_path
            
            # Act: Check credentials
            is_valid, error_msg = ensure_no_long_lived_credentials()
            
            # Assert: Should accept IRSA credentials
            self.assertTrue(is_valid)
            self.assertIsNone(error_msg)
        finally:
            # Cleanup
            os.unlink(token_file_path)
    
    def test_property_reject_missing_irsa_configuration(self):
        """
        Property: When neither long-lived credentials nor IRSA are configured,
        ensure_no_long_lived_credentials should return False
        """
        # Arrange: Clear all credential-related environment variables
        os.environ.pop('AWS_ACCESS_KEY_ID', None)
        os.environ.pop('AWS_SECRET_ACCESS_KEY', None)
        os.environ.pop('AWS_ROLE_ARN', None)
        os.environ.pop('AWS_WEB_IDENTITY_TOKEN_FILE', None)
        
        # Act: Check credentials
        is_valid, error_msg = ensure_no_long_lived_credentials()
        
        # Assert: Should reject (no credentials configured)
        self.assertFalse(is_valid)
        self.assertIsNotNone(error_msg)
    
    @given(
        role_arn=iam_role_arn(),
        access_key=aws_access_key()
    )
    @settings(max_examples=100)
    def test_property_reject_mixed_credentials(self, role_arn, access_key):
        """
        Property: For any combination of IRSA and long-lived credentials,
        ensure_no_long_lived_credentials should reject if long-lived credentials are present
        
        This ensures that even if IRSA is configured, long-lived credentials take precedence
        in the rejection logic (security-first approach)
        """
        # Arrange: Set both IRSA and long-lived credentials
        with tempfile.NamedTemporaryFile(mode='w', delete=False) as token_file:
            token_file.write('mock-token-content')
            token_file_path = token_file.name
        
        try:
            os.environ['AWS_ROLE_ARN'] = role_arn
            os.environ['AWS_WEB_IDENTITY_TOKEN_FILE'] = token_file_path
            os.environ['AWS_ACCESS_KEY_ID'] = access_key
            
            # Act: Check credentials
            is_valid, error_msg = ensure_no_long_lived_credentials()
            
            # Assert: Should reject due to long-lived credentials
            self.assertFalse(is_valid)
            self.assertIsNotNone(error_msg)
            self.assertIn('Long-lived', error_msg)
        finally:
            # Cleanup
            os.unlink(token_file_path)


if __name__ == '__main__':
    unittest.main()
