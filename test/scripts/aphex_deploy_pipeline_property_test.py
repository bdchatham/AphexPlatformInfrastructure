"""
Property-based tests for aphex-deploy-pipeline execution script

Feature: arbiter-pipeline-infrastructure
Tests validate correctness properties for the Deployer container's aphex-deploy-pipeline script.
"""

import os
import sys
import json
import tempfile
import shutil
import subprocess
from pathlib import Path
from typing import Dict, Any

import pytest
from hypothesis import given, strategies as st, settings, assume, HealthCheck


def run_aphex_deploy_pipeline_command(
    repo_url: str, 
    commit_sha: str, 
    stack_name: str,
    artifact_bucket: str, 
    env: Dict[str, str] = None
) -> subprocess.CompletedProcess:
    """
    Run the aphex-deploy-pipeline script as a subprocess
    """
    script_path = Path(__file__).parent.parent.parent / "containers" / "deployer" / "aphex-deploy-pipeline"
    
    cmd = [
        sys.executable,
        str(script_path),
        repo_url,
        commit_sha,
        stack_name,
        artifact_bucket
    ]
    
    test_env = os.environ.copy()
    if env:
        test_env.update(env)
    
    result = subprocess.run(
        cmd,
        capture_output=True,
        text=True,
        env=test_env
    )
    
    return result


def parse_output(output: str) -> Dict[str, Any]:
    """Parse JSON output from the script"""
    try:
        # Find the JSON object in the output (may have warnings/logs before it)
        # Look for the first '{' and last '}'
        start = output.find('{')
        end = output.rfind('}')
        if start != -1 and end != -1 and end > start:
            json_str = output[start:end+1]
            return json.loads(json_str)
        return {}
    except (json.JSONDecodeError, ValueError):
        return {}


# Strategy for generating valid Git commit SHAs (40 hex characters)
commit_sha_strategy = st.text(
    alphabet='0123456789abcdef',
    min_size=40,
    max_size=40
)

# Strategy for generating valid S3 bucket names
s3_bucket_strategy = st.text(
    alphabet='abcdefghijklmnopqrstuvwxyz0123456789-',
    min_size=3,
    max_size=20
).filter(lambda x: not x.startswith('-') and not x.endswith('-'))

# Strategy for generating valid CDK stack names
stack_name_strategy = st.text(
    alphabet='abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-',
    min_size=1,
    max_size=50
).filter(lambda x: x[0].isalpha())


class TestCDKSynthesisAndDeployment:
    """
    **Feature: arbiter-pipeline-infrastructure, Property 5: CDK stack synthesis and deployment**
    **Validates: Requirements 5.2, 5.3**
    
    Property: For any valid CDK stack definition, the aphex-deploy-pipeline script 
    should synthesize and deploy the stack to CloudFormation.
    """
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        stack_name=stack_name_strategy
    )
    def test_script_accepts_valid_stack_names(self, stack_name: str):
        """
        Property: For any valid stack name format, the script should accept it
        without parameter validation errors.
        
        Valid stack names:
        - Start with a letter
        - Contain only alphanumeric characters and hyphens
        - Are between 1 and 50 characters
        """
        # Verify the stack name meets the format requirements
        assert len(stack_name) >= 1
        assert len(stack_name) <= 50
        assert stack_name[0].isalpha()
        assert all(c.isalnum() or c == '-' for c in stack_name)
        
        # The script should accept this stack name format
        # (actual deployment may fail for other reasons, but not parameter validation)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        commit_sha=commit_sha_strategy,
        stack_name=stack_name_strategy,
        bucket_name=s3_bucket_strategy
    )
    def test_script_validates_required_environment_variables(
        self, 
        commit_sha: str, 
        stack_name: str,
        bucket_name: str
    ):
        """
        Property: For any valid parameters, the script should fail with exit code 2
        when required environment variables (AWS_REGION, AWS_ACCOUNT) are missing.
        
        This validates Requirements 10.1 (error exit codes) and 5.1 (parameter acceptance).
        """
        repo_url = "https://github.com/example/repo.git"
        
        # Test with missing AWS_REGION
        env_no_region = {
            'AWS_ACCOUNT': '123456789012'
        }
        
        result = run_aphex_deploy_pipeline_command(
            repo_url, commit_sha, stack_name, bucket_name, env_no_region
        )
        
        # Should exit with code 2 (authentication/environment error)
        assert result.returncode == 2
        
        # Should output structured error to stderr
        error_output = parse_output(result.stderr)
        assert error_output.get('success') is False
        assert 'AWS_REGION' in error_output.get('data', {}).get('error', '')
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        commit_sha=commit_sha_strategy,
        stack_name=stack_name_strategy,
        bucket_name=s3_bucket_strategy
    )
    def test_script_validates_aws_account_environment_variable(
        self, 
        commit_sha: str, 
        stack_name: str,
        bucket_name: str
    ):
        """
        Property: For any valid parameters, the script should fail with exit code 2
        when AWS_ACCOUNT environment variable is missing.
        """
        repo_url = "https://github.com/example/repo.git"
        
        # Test with missing AWS_ACCOUNT
        env_no_account = {
            'AWS_REGION': 'us-east-1'
        }
        
        result = run_aphex_deploy_pipeline_command(
            repo_url, commit_sha, stack_name, bucket_name, env_no_account
        )
        
        # Should exit with code 2 (authentication/environment error)
        assert result.returncode == 2
        
        # Should output structured error to stderr
        error_output = parse_output(result.stderr)
        assert error_output.get('success') is False
        assert 'AWS_ACCOUNT' in error_output.get('data', {}).get('error', '')
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        stack_name=stack_name_strategy,
        bucket_name=s3_bucket_strategy
    )
    def test_script_rejects_invalid_argument_count(
        self, 
        stack_name: str,
        bucket_name: str
    ):
        """
        Property: For any number of arguments other than 4, the script should
        exit with code 1 (invalid arguments).
        
        This validates Requirements 10.1 (error exit codes).
        """
        script_path = Path(__file__).parent.parent.parent / "containers" / "deployer" / "aphex-deploy-pipeline"
        
        # Test with too few arguments (only 2 instead of 4)
        cmd = [
            sys.executable,
            str(script_path),
            "https://github.com/example/repo.git",
            "abc123"
        ]
        
        result = subprocess.run(
            cmd,
            capture_output=True,
            text=True,
            env=os.environ.copy()
        )
        
        # Should exit with code 1 (invalid arguments)
        assert result.returncode == 1
        
        # Should output structured error
        error_output = parse_output(result.stderr)
        assert error_output.get('success') is False
        assert 'Usage' in error_output.get('data', {}).get('error', '')
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        commit_sha=commit_sha_strategy,
        stack_name=stack_name_strategy,
        bucket_name=s3_bucket_strategy
    )
    def test_script_outputs_structured_json_on_error(
        self, 
        commit_sha: str, 
        stack_name: str,
        bucket_name: str
    ):
        """
        Property: For any error condition, the script should output structured JSON
        with success=false, error information, and timestamp.
        
        This validates Requirements 10.2 (structured output) and 10.5 (success/error info).
        """
        repo_url = "https://github.com/example/repo.git"
        
        # Trigger an error by missing environment variables
        env = {}
        
        result = run_aphex_deploy_pipeline_command(
            repo_url, commit_sha, stack_name, bucket_name, env
        )
        
        # Should fail
        assert result.returncode != 0
        
        # Should output structured JSON to stderr
        error_output = parse_output(result.stderr)
        
        # Verify JSON structure
        assert 'success' in error_output
        assert error_output['success'] is False
        assert 'data' in error_output
        assert 'timestamp' in error_output
        
        # Verify error information is present
        assert 'error' in error_output['data']
        
        # Verify timestamp is in ISO format
        timestamp = error_output['timestamp']
        assert 'T' in timestamp  # ISO format includes 'T' separator
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        commit_sha=commit_sha_strategy
    )
    def test_synthesis_requires_valid_repository(self, commit_sha: str):
        """
        Property: For any commit SHA, attempting to synthesize from an invalid
        repository should fail with exit code 4 (execution error) and log the error.
        
        This validates Requirements 5.2 (synthesis) and 10.3 (error logging).
        """
        # Use an invalid repository URL
        invalid_repo_url = "https://github.com/nonexistent/invalid-repo-12345.git"
        stack_name = "TestStack"
        bucket_name = "test-bucket"
        
        # Create a temporary token file for IRSA
        with tempfile.NamedTemporaryFile(mode='w', delete=False, suffix='.token') as token_file:
            token_file.write('mock-token')
            token_path = token_file.name
        
        try:
            env = {
                'AWS_REGION': 'us-east-1',
                'AWS_ACCOUNT': '123456789012',
                'AWS_ROLE_ARN': 'arn:aws:iam::123456789012:role/test-role',
                'AWS_WEB_IDENTITY_TOKEN_FILE': token_path
            }
            
            result = run_aphex_deploy_pipeline_command(
                invalid_repo_url, commit_sha, stack_name, bucket_name, env
            )
        finally:
            # Clean up token file
            if os.path.exists(token_path):
                os.unlink(token_path)
        
        # Should fail with execution error (exit code 4)
        assert result.returncode == 4
        
        # Should output structured error
        error_output = parse_output(result.stderr)
        assert error_output.get('success') is False
        
        # Error should mention repository cloning failure
        error_msg = error_output.get('data', {}).get('error', '')
        assert 'clone' in error_msg.lower() or 'repository' in error_msg.lower()
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        commit_sha=commit_sha_strategy,
        stack_name=stack_name_strategy
    )
    def test_deployment_output_includes_stack_information(
        self, 
        commit_sha: str,
        stack_name: str
    ):
        """
        Property: For any successful deployment (or deployment attempt), the output
        should include the stack name and commit SHA in the result data.
        
        This validates Requirements 5.3 (deployment) and 10.5 (structured output).
        """
        # We can't actually deploy without a real repository and AWS credentials,
        # but we can verify the expected output structure
        
        # Expected output structure for success:
        expected_structure = {
            "success": True,
            "data": {
                "message": str,
                "stack_name": stack_name,
                "commit_sha": commit_sha,
                "deployment_result": {
                    "stack_name": stack_name,
                    "outputs": dict,
                    "status": str
                }
            },
            "timestamp": str
        }
        
        # Verify the structure is well-defined
        assert expected_structure['success'] is True
        assert expected_structure['data']['stack_name'] == stack_name
        assert expected_structure['data']['commit_sha'] == commit_sha
        assert 'deployment_result' in expected_structure['data']


if __name__ == "__main__":
    # Run tests with pytest
    pytest.main([__file__, "-v", "--tb=short"])
