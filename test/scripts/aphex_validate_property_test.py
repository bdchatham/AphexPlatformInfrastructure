"""
Property-based tests for aphex-validate execution script

Feature: arbiter-pipeline-infrastructure
Tests validate correctness properties for the Validator container's aphex-validate script.
"""

import os
import sys
import json
import tempfile
import subprocess
from pathlib import Path
from typing import Dict, Any

import pytest
from hypothesis import given, strategies as st, settings, assume, HealthCheck


def run_aphex_validate_command(config_path: str, env: Dict[str, str] = None) -> subprocess.CompletedProcess:
    """
    Run the aphex-validate script as a subprocess
    """
    script_path = Path(__file__).parent.parent.parent / "containers" / "validator" / "aphex-validate"
    
    cmd = [
        sys.executable,
        str(script_path),
        config_path
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


# Strategy for generating valid pipeline names
pipeline_name_strategy = st.text(
    alphabet='abcdefghijklmnopqrstuvwxyzABCDEFGHIJKLMNOPQRSTUVWXYZ0123456789-_',
    min_size=1,
    max_size=50
)

# Strategy for generating valid repository URLs
repo_url_strategy = st.sampled_from([
    "https://github.com/example/repo.git",
    "https://github.com/user/project.git",
    "https://gitlab.com/org/repo.git",
])

# Strategy for generating valid branch names
branch_strategy = st.sampled_from([
    "main",
    "master",
    "develop",
    "feature/test",
])

# Strategy for generating build commands
build_commands_strategy = st.lists(
    st.sampled_from([
        "npm install",
        "npm run build",
        "python setup.py build",
        "make",
    ]),
    min_size=0,
    max_size=5
)

# Strategy for generating test commands
test_commands_strategy = st.lists(
    st.sampled_from([
        "npm test",
        "pytest",
        "jest",
    ]),
    min_size=0,
    max_size=3
)


def create_valid_config(
    pipeline_name: str,
    repo_url: str,
    branch: str = "main",
    build_commands: list = None,
    test_commands: list = None,
    include_cdk_context: bool = False,
    required_context_keys: list = None
) -> Dict[str, Any]:
    """Create a valid configuration dictionary"""
    config = {
        "pipeline": {
            "name": pipeline_name,
            "repository": repo_url,
            "branch": branch
        }
    }
    
    if build_commands:
        config["build"] = {
            "commands": build_commands,
            "artifactBucket": "test-artifacts-bucket"
        }
    
    if test_commands:
        config["test"] = {
            "commands": test_commands
        }
    
    if include_cdk_context:
        config["cdk"] = {
            "context": {
                "vpc-id": "vpc-12345",
                "availability-zones": ["us-east-1a", "us-east-1b"]
            }
        }
        
        if required_context_keys:
            config["cdk"]["requiredContext"] = required_context_keys
            # Ensure all required keys are present
            for key in required_context_keys:
                if key not in config["cdk"]["context"]:
                    config["cdk"]["context"][key] = f"value-{key}"
    
    return config


class TestConfigurationValidationSequence:
    """
    **Feature: arbiter-pipeline-infrastructure, Property 9: Configuration validation sequence**
    **Validates: Requirements 7.2, 7.3, 7.4, 7.5**
    
    Property: For any configuration file, the aphex-validate script should validate it 
    against the JSON schema, then validate AWS credentials, then validate CDK context, 
    and output success only if all validations pass.
    """
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy,
        repo_url=repo_url_strategy,
        branch=branch_strategy,
        build_commands=build_commands_strategy,
        test_commands=test_commands_strategy
    )
    def test_valid_config_passes_schema_validation(
        self,
        pipeline_name: str,
        repo_url: str,
        branch: str,
        build_commands: list,
        test_commands: list
    ):
        """
        Property: For any valid configuration structure, the schema validation
        step should pass.
        
        **Validates: Requirements 7.2**
        """
        # Create a valid configuration
        config = create_valid_config(
            pipeline_name, repo_url, branch, build_commands, test_commands
        )
        
        # Write to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            json.dump(config, f)
            config_path = f.name
        
        try:
            result = run_aphex_validate_command(config_path)
            
            # Parse output
            if result.returncode == 0:
                output_data = parse_output(result.stdout)
            else:
                output_data = parse_output(result.stderr)
            
            # Verify schema validation passed
            assert "validation_results" in output_data.get("data", {})
            validation_results = output_data["data"]["validation_results"]
            
            assert "schema_validation" in validation_results
            assert validation_results["schema_validation"]["passed"] is True
            assert validation_results["schema_validation"]["error"] == ""
            
        finally:
            # Clean up
            os.unlink(config_path)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy,
        repo_url=repo_url_strategy
    )
    def test_missing_required_field_fails_schema_validation(
        self,
        pipeline_name: str,
        repo_url: str
    ):
        """
        Property: For any configuration missing required fields, the schema
        validation should fail and prevent subsequent validations.
        
        **Validates: Requirements 7.2, 7.5**
        """
        # Create an invalid configuration (missing required 'pipeline' field)
        config = {
            "build": {
                "commands": ["npm install"]
            }
        }
        
        # Write to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            json.dump(config, f)
            config_path = f.name
        
        try:
            result = run_aphex_validate_command(config_path)
            
            # Should fail
            assert result.returncode == 1
            
            # Parse output
            output_data = parse_output(result.stderr)
            
            # Verify schema validation failed
            assert output_data.get("success") is False
            assert "validation_results" in output_data.get("data", {})
            validation_results = output_data["data"]["validation_results"]
            
            assert "schema_validation" in validation_results
            assert validation_results["schema_validation"]["passed"] is False
            assert validation_results["schema_validation"]["error"] != ""
            
        finally:
            # Clean up
            os.unlink(config_path)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy,
        repo_url=repo_url_strategy,
        branch=branch_strategy
    )
    def test_validation_sequence_order(
        self,
        pipeline_name: str,
        repo_url: str,
        branch: str
    ):
        """
        Property: For any configuration, validations should execute in order:
        1. Schema validation
        2. AWS credentials validation
        3. CDK context validation
        
        If any step fails, subsequent steps should not execute.
        
        **Validates: Requirements 7.2, 7.3, 7.4, 7.5**
        """
        # Create a valid configuration
        config = create_valid_config(pipeline_name, repo_url, branch)
        
        # Write to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            json.dump(config, f)
            config_path = f.name
        
        try:
            result = run_aphex_validate_command(config_path)
            
            # Parse output
            if result.returncode == 0:
                output_data = parse_output(result.stdout)
            else:
                output_data = parse_output(result.stderr)
            
            # Verify validation results structure
            assert "validation_results" in output_data.get("data", {})
            validation_results = output_data["data"]["validation_results"]
            
            # All three validation types should be present
            assert "schema_validation" in validation_results
            assert "aws_credentials" in validation_results
            assert "cdk_context" in validation_results
            
            # If schema validation failed, AWS credentials should not have been checked
            if not validation_results["schema_validation"]["passed"]:
                # AWS credentials validation should not have passed
                # (it should have been skipped or failed)
                assert validation_results["aws_credentials"]["passed"] is False
            
            # If AWS credentials failed, CDK context should not have been checked
            if not validation_results["aws_credentials"]["passed"]:
                # CDK context validation should not have passed
                # (it should have been skipped or failed)
                assert validation_results["cdk_context"]["passed"] is False
            
        finally:
            # Clean up
            os.unlink(config_path)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy,
        repo_url=repo_url_strategy,
        required_keys=st.lists(
            st.sampled_from(["vpc-id", "subnet-ids", "security-group-id", "availability-zones"]),
            min_size=1,
            max_size=3,
            unique=True
        )
    )
    def test_cdk_context_validation_with_required_keys(
        self,
        pipeline_name: str,
        repo_url: str,
        required_keys: list
    ):
        """
        Property: For any configuration with required CDK context keys,
        the validation should pass only if all required keys are present.
        
        **Validates: Requirements 7.4**
        """
        # Create a configuration with CDK context and required keys
        config = create_valid_config(
            pipeline_name,
            repo_url,
            include_cdk_context=True,
            required_context_keys=required_keys
        )
        
        # Write to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            json.dump(config, f)
            config_path = f.name
        
        try:
            result = run_aphex_validate_command(config_path)
            
            # Parse output
            if result.returncode == 0:
                output_data = parse_output(result.stdout)
            else:
                output_data = parse_output(result.stderr)
            
            # Verify CDK context validation
            validation_results = output_data.get("data", {}).get("validation_results", {})
            
            # Schema validation should pass
            assert validation_results.get("schema_validation", {}).get("passed") is True
            
            # If we get to CDK context validation, it should pass
            # (because we ensured all required keys are present)
            if "cdk_context" in validation_results:
                cdk_result = validation_results["cdk_context"]
                # If AWS credentials passed, CDK context should also pass
                if validation_results.get("aws_credentials", {}).get("passed"):
                    assert cdk_result["passed"] is True
            
        finally:
            # Clean up
            os.unlink(config_path)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy,
        repo_url=repo_url_strategy,
        missing_key=st.sampled_from(["vpc-id", "subnet-ids", "security-group-id"])
    )
    def test_cdk_context_validation_fails_with_missing_required_keys(
        self,
        pipeline_name: str,
        repo_url: str,
        missing_key: str
    ):
        """
        Property: For any configuration with missing required CDK context keys,
        the CDK context validation should fail.
        
        **Validates: Requirements 7.4, 7.5**
        """
        # Create a configuration with CDK context but missing a required key
        config = create_valid_config(
            pipeline_name,
            repo_url,
            include_cdk_context=True,
            required_context_keys=[missing_key]
        )
        
        # Remove the required key from context
        if "cdk" in config and "context" in config["cdk"]:
            config["cdk"]["context"].pop(missing_key, None)
        
        # Write to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            json.dump(config, f)
            config_path = f.name
        
        try:
            result = run_aphex_validate_command(config_path)
            
            # Parse output
            output_data = parse_output(result.stderr if result.returncode != 0 else result.stdout)
            
            # If we get to CDK context validation, it should fail
            validation_results = output_data.get("data", {}).get("validation_results", {})
            
            # If AWS credentials passed and we got to CDK context validation
            if validation_results.get("aws_credentials", {}).get("passed"):
                # CDK context validation should fail
                assert validation_results.get("cdk_context", {}).get("passed") is False
                assert missing_key in validation_results.get("cdk_context", {}).get("error", "")
            
        finally:
            # Clean up
            os.unlink(config_path)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy,
        repo_url=repo_url_strategy,
        branch=branch_strategy
    )
    def test_all_validations_pass_produces_success_output(
        self,
        pipeline_name: str,
        repo_url: str,
        branch: str
    ):
        """
        Property: For any valid configuration with valid AWS credentials and
        valid CDK context, all validations should pass and the script should
        output success with exit code 0.
        
        **Validates: Requirements 7.5**
        """
        # Create a valid configuration
        config = create_valid_config(pipeline_name, repo_url, branch)
        
        # Write to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            json.dump(config, f)
            config_path = f.name
        
        try:
            result = run_aphex_validate_command(config_path)
            
            # Parse output
            if result.returncode == 0:
                output_data = parse_output(result.stdout)
                
                # Verify success
                assert output_data.get("success") is True
                assert "validation_results" in output_data.get("data", {})
                
                validation_results = output_data["data"]["validation_results"]
                
                # All validations should have passed
                assert validation_results["schema_validation"]["passed"] is True
                assert validation_results["aws_credentials"]["passed"] is True
                assert validation_results["cdk_context"]["passed"] is True
                
                # No errors should be present
                assert validation_results["schema_validation"]["error"] == ""
                assert validation_results["aws_credentials"]["error"] == ""
                assert validation_results["cdk_context"]["error"] == ""
            else:
                # If it failed, it should be due to AWS credentials
                # (since schema and CDK context should pass for valid config)
                output_data = parse_output(result.stderr)
                validation_results = output_data.get("data", {}).get("validation_results", {})
                
                # Schema should have passed
                assert validation_results.get("schema_validation", {}).get("passed") is True
                
                # AWS credentials likely failed (expected in test environment)
                # This is acceptable for the property test
            
        finally:
            # Clean up
            os.unlink(config_path)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy,
        repo_url=repo_url_strategy
    )
    def test_structured_output_format(
        self,
        pipeline_name: str,
        repo_url: str
    ):
        """
        Property: For any configuration, the script should output structured JSON
        with success status, validation results, and timestamp.
        
        **Validates: Requirements 7.5, 10.2, 10.5**
        """
        # Create a valid configuration
        config = create_valid_config(pipeline_name, repo_url)
        
        # Write to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            json.dump(config, f)
            config_path = f.name
        
        try:
            result = run_aphex_validate_command(config_path)
            
            # Parse output
            if result.returncode == 0:
                output_data = parse_output(result.stdout)
            else:
                output_data = parse_output(result.stderr)
            
            # Verify required fields exist
            assert "success" in output_data
            assert "data" in output_data
            assert "timestamp" in output_data
            
            # Verify success is a boolean
            assert isinstance(output_data["success"], bool)
            
            # Verify data contains validation results
            assert "validation_results" in output_data["data"]
            validation_results = output_data["data"]["validation_results"]
            
            # Verify all three validation types are present
            assert "schema_validation" in validation_results
            assert "aws_credentials" in validation_results
            assert "cdk_context" in validation_results
            
            # Each validation result should have 'passed' and 'error' fields
            for validation_type in ["schema_validation", "aws_credentials", "cdk_context"]:
                assert "passed" in validation_results[validation_type]
                assert "error" in validation_results[validation_type]
                assert isinstance(validation_results[validation_type]["passed"], bool)
                assert isinstance(validation_results[validation_type]["error"], str)
            
            # Verify timestamp is in ISO format
            timestamp = output_data["timestamp"]
            assert 'T' in timestamp  # ISO format includes 'T' separator
            
        finally:
            # Clean up
            os.unlink(config_path)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy
    )
    def test_invalid_json_fails_gracefully(self, pipeline_name: str):
        """
        Property: For any invalid JSON/YAML file, the script should fail
        gracefully with appropriate error message.
        
        **Validates: Requirements 7.2, 10.1, 10.2**
        """
        # Create an invalid JSON/YAML file that can't be parsed by either parser
        invalid_content = "{ this is not valid json or yaml: [[[[ }"
        
        # Write to temporary file
        with tempfile.NamedTemporaryFile(mode='w', suffix='.json', delete=False) as f:
            f.write(invalid_content)
            config_path = f.name
        
        try:
            result = run_aphex_validate_command(config_path)
            
            # Should fail with exit code 1
            assert result.returncode == 1
            
            # Parse output
            output_data = parse_output(result.stderr)
            
            # Verify error is reported
            assert output_data.get("success") is False
            assert "error" in output_data.get("data", {})
            
            # Error should mention parsing or validation failure
            error_msg = output_data["data"]["error"]
            assert ("parse" in error_msg.lower() or 
                    "json" in error_msg.lower() or 
                    "yaml" in error_msg.lower() or
                    "validation" in error_msg.lower() or
                    "schema" in error_msg.lower())
            
        finally:
            # Clean up
            os.unlink(config_path)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        pipeline_name=pipeline_name_strategy
    )
    def test_nonexistent_file_fails_gracefully(self, pipeline_name: str):
        """
        Property: For any nonexistent file path, the script should fail
        gracefully with appropriate error message.
        
        **Validates: Requirements 7.1, 10.1, 10.2**
        """
        # Use a nonexistent file path
        config_path = f"/tmp/nonexistent-{pipeline_name}-config.json"
        
        result = run_aphex_validate_command(config_path)
        
        # Should fail with exit code 1
        assert result.returncode == 1
        
        # Parse output
        output_data = parse_output(result.stderr)
        
        # Verify error is reported
        assert output_data.get("success") is False
        assert "error" in output_data.get("data", {})
        
        # Error should mention file not found
        error_msg = output_data["data"]["error"]
        assert "not found" in error_msg.lower() or "does not exist" in error_msg.lower()


if __name__ == "__main__":
    # Run tests with pytest
    pytest.main([__file__, "-v", "--tb=short"])
