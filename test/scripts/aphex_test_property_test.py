"""
Property-based tests for aphex-test execution script

Feature: arbiter-pipeline-infrastructure
Tests validate correctness properties for the Tester container's aphex-test script.
"""

import os
import sys
import json
import subprocess
from pathlib import Path
from typing import Dict, Any

import pytest
from hypothesis import given, strategies as st, settings, assume, HealthCheck


def run_aphex_test_command(test_commands: str, env: Dict[str, str] = None) -> subprocess.CompletedProcess:
    """
    Run the aphex-test script as a subprocess
    """
    script_path = Path(__file__).parent.parent.parent / "containers" / "tester" / "aphex-test"
    
    cmd = [
        sys.executable,
        str(script_path),
        test_commands
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
        # The output may contain logging lines before the JSON
        # Find the JSON object by looking for the opening brace
        lines = output.strip().split('\n')
        json_lines = []
        in_json = False
        
        for line in lines:
            if line.strip().startswith('{'):
                in_json = True
            if in_json:
                json_lines.append(line)
        
        if json_lines:
            json_str = '\n'.join(json_lines)
            return json.loads(json_str)
        
        return {}
    except json.JSONDecodeError:
        return {}


# Strategy for generating valid exit codes (0-255)
exit_code_strategy = st.integers(min_value=0, max_value=255)

# Strategy for generating test commands
simple_command_strategy = st.sampled_from([
    "echo 'test passed'",
    "true",
    "python3 -c 'print(\"test\")'",
    "node -e 'console.log(\"test\")'",
])

failing_command_strategy = st.sampled_from([
    "false",
    "exit 1",
    "python3 -c 'import sys; sys.exit(1)'",
    "node -e 'process.exit(1)'",
])


class TestTestExecution:
    """
    **Feature: arbiter-pipeline-infrastructure, Property 8: Test execution and result capture**
    **Validates: Requirements 6.2, 6.3, 6.4, 6.5**
    
    Property: For any test commands, the aphex-test script should execute them,
    capture stdout/stderr, capture the exit code, and output pass/fail status
    based on the exit code.
    """
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        exit_code=exit_code_strategy
    )
    def test_exit_code_determines_pass_fail_status(self, exit_code: int):
        """
        Property: For any test command with a specific exit code, the script
        should report "pass" status if exit code is 0, and "fail" status otherwise.
        
        **Validates: Requirements 6.4, 6.5**
        """
        # Create a command that exits with the specified code
        test_command = f"exit {exit_code}"
        
        result = run_aphex_test_command(test_command)
        
        # Parse the output
        if result.returncode == 0:
            output_data = parse_output(result.stdout)
        else:
            output_data = parse_output(result.stderr)
        
        # Verify structured output exists
        assert "success" in output_data
        assert "data" in output_data
        assert "timestamp" in output_data
        
        # Verify the status matches the exit code
        if exit_code == 0:
            # Exit code 0 should result in "pass" status
            assert output_data["success"] is True
            assert output_data["data"]["test_results"]["status"] == "pass"
            assert output_data["data"]["test_results"]["exit_code"] == 0
            assert result.returncode == 0
        else:
            # Non-zero exit code should result in "fail" status
            assert output_data["success"] is False
            assert output_data["data"]["test_results"]["status"] == "fail"
            assert output_data["data"]["test_results"]["exit_code"] == exit_code
            assert result.returncode == exit_code
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        command=simple_command_strategy
    )
    def test_stdout_capture(self, command: str):
        """
        Property: For any test command that produces stdout, the script
        should capture and include it in the output.
        
        **Validates: Requirements 6.2, 6.3**
        """
        result = run_aphex_test_command(command)
        
        # Parse the output
        output_data = parse_output(result.stdout)
        
        # Verify stdout is captured
        assert "data" in output_data
        assert "test_results" in output_data["data"]
        assert "stdout" in output_data["data"]["test_results"]
        
        # The stdout field should be a string (may be empty or contain output)
        assert isinstance(output_data["data"]["test_results"]["stdout"], str)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        command=failing_command_strategy
    )
    def test_stderr_capture(self, command: str):
        """
        Property: For any test command that produces stderr or fails, the script
        should capture stderr and include it in the output.
        
        **Validates: Requirements 6.2, 6.3**
        """
        result = run_aphex_test_command(command)
        
        # Parse the output (will be in stderr for failed commands)
        output_data = parse_output(result.stderr)
        
        # Verify stderr is captured
        assert "data" in output_data
        assert "test_results" in output_data["data"]
        assert "stderr" in output_data["data"]["test_results"]
        
        # The stderr field should be a string
        assert isinstance(output_data["data"]["test_results"]["stderr"], str)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        exit_code=exit_code_strategy
    )
    def test_exit_code_capture(self, exit_code: int):
        """
        Property: For any test command, the script should capture and report
        the exact exit code in the structured output.
        
        **Validates: Requirements 6.4**
        """
        test_command = f"exit {exit_code}"
        
        result = run_aphex_test_command(test_command)
        
        # Parse the output
        if result.returncode == 0:
            output_data = parse_output(result.stdout)
        else:
            output_data = parse_output(result.stderr)
        
        # Verify exit code is captured
        assert "data" in output_data
        assert "test_results" in output_data["data"]
        assert "exit_code" in output_data["data"]["test_results"]
        
        # The captured exit code should match the command's exit code
        assert output_data["data"]["test_results"]["exit_code"] == exit_code
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        command=st.sampled_from([
            simple_command_strategy,
            failing_command_strategy
        ]).flatmap(lambda s: s)
    )
    def test_structured_json_output(self, command: str):
        """
        Property: For any test command, the script should output structured JSON
        with success status, data, and timestamp.
        
        **Validates: Requirements 6.2, 6.3, 6.4, 6.5**
        """
        result = run_aphex_test_command(command)
        
        # Parse the output (try stdout first, then stderr)
        output_data = parse_output(result.stdout)
        if not output_data:
            output_data = parse_output(result.stderr)
        
        # Verify required fields exist
        assert "success" in output_data
        assert "data" in output_data
        assert "timestamp" in output_data
        
        # Verify data structure
        assert "test_results" in output_data["data"]
        test_results = output_data["data"]["test_results"]
        
        assert "status" in test_results
        assert "exit_code" in test_results
        assert "stdout" in test_results
        assert "stderr" in test_results
        assert "commands_executed" in test_results
        
        # Verify status is either "pass" or "fail"
        assert test_results["status"] in ["pass", "fail"]
        
        # Verify exit_code is an integer
        assert isinstance(test_results["exit_code"], int)
        
        # Verify stdout and stderr are strings
        assert isinstance(test_results["stdout"], str)
        assert isinstance(test_results["stderr"], str)
        
        # Verify commands_executed is a list
        assert isinstance(test_results["commands_executed"], list)
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        num_commands=st.integers(min_value=1, max_value=5)
    )
    def test_multiple_command_execution(self, num_commands: int):
        """
        Property: For any sequence of test commands, the script should execute
        all of them and capture results from all commands.
        
        **Validates: Requirements 6.2, 6.3**
        """
        # Create multiple simple commands
        commands = [f"echo 'test {i}'" for i in range(num_commands)]
        test_command = "; ".join(commands)
        
        result = run_aphex_test_command(test_command)
        
        # Parse the output
        output_data = parse_output(result.stdout)
        
        # Verify all commands were executed
        assert "data" in output_data
        assert "test_results" in output_data["data"]
        assert "commands_executed" in output_data["data"]["test_results"]
        
        # The number of executed commands should match
        executed = output_data["data"]["test_results"]["commands_executed"]
        assert len(executed) == num_commands
    
    @settings(
        max_examples=100,
        deadline=None,
        suppress_health_check=[HealthCheck.function_scoped_fixture]
    )
    @given(
        failing_position=st.integers(min_value=0, max_value=2)
    )
    def test_failure_propagation_in_command_sequence(self, failing_position: int):
        """
        Property: For any sequence of commands where one fails, the script
        should report overall failure status even if other commands succeed.
        
        **Validates: Requirements 6.4, 6.5**
        """
        # Create a sequence with one failing command
        commands = ["true", "true", "true"]
        commands[failing_position] = "false"
        test_command = "; ".join(commands)
        
        result = run_aphex_test_command(test_command)
        
        # Parse the output
        output_data = parse_output(result.stderr)
        
        # Verify failure is reported
        assert output_data["success"] is False
        assert output_data["data"]["test_results"]["status"] == "fail"
        assert output_data["data"]["test_results"]["exit_code"] != 0


if __name__ == "__main__":
    # Run tests with pytest
    pytest.main([__file__, "-v", "--tb=short"])
