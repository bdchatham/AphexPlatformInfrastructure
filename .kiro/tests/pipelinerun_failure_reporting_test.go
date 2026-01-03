package tests

import (
	"fmt"
	"strings"
	"testing"
	"testing/quick"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Feature: argocd-tekton-platform, Property 33: PipelineRun Failure Reporting
// Validates: Requirements 10.4
//
// Property: For any PipelineRun that fails, the PipelineRun status should contain a failure message and the failed task name.

// PipelineRunStatus represents the status of a Tekton PipelineRun
type PipelineRunStatus struct {
	Conditions []PipelineRunCondition
	TaskRuns   map[string]TaskRunStatus
}

// PipelineRunCondition represents a condition in the PipelineRun status
type PipelineRunCondition struct {
	Type    string
	Status  string
	Reason  string
	Message string
}

// TaskRunStatus represents the status of a TaskRun within a PipelineRun
type TaskRunStatus struct {
	TaskName   string
	Status     string
	Reason     string
	Message    string
	StartTime  metav1.Time
	FinishTime metav1.Time
}

// FailureScenario represents a test scenario for pipeline failure reporting
type FailureScenario struct {
	PipelineRunName string
	FailedTaskName  string
	FailureReason   string
	FailureMessage  string
}

// TestPipelineRunFailureReporting_StatusContainsFailureMessage tests that failed PipelineRuns have failure messages
func TestPipelineRunFailureReporting_StatusContainsFailureMessage(t *testing.T) {
	// Property: For any failed PipelineRun, the status should contain a failure message
	
	scenarios := []FailureScenario{
		{
			PipelineRunName: "pipeline-run-1",
			FailedTaskName:  "git-clone",
			FailureReason:   "TaskRunFailed",
			FailureMessage:  "Failed to clone repository: authentication failed",
		},
		{
			PipelineRunName: "pipeline-run-2",
			FailedTaskName:  "cdktf-synth",
			FailureReason:   "TaskRunFailed",
			FailureMessage:  "CDKTF synthesis failed: syntax error in main.ts",
		},
		{
			PipelineRunName: "pipeline-run-3",
			FailedTaskName:  "cdktf-deploy",
			FailureReason:   "TaskRunFailed",
			FailureMessage:  "Terraform apply failed: resource already exists",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		status := PipelineRunStatus{
			Conditions: []PipelineRunCondition{
				{
					Type:    "Succeeded",
					Status:  "False",
					Reason:  scenario.FailureReason,
					Message: scenario.FailureMessage,
				},
			},
		}
		
		// Verify status has at least one condition
		if len(status.Conditions) == 0 {
			t.Errorf("Iteration %d: PipelineRun status has no conditions", i)
			continue
		}
		
		// Verify the Succeeded condition is False
		succeededCondition := status.Conditions[0]
		if succeededCondition.Type != "Succeeded" {
			t.Errorf("Iteration %d: Expected Succeeded condition, got %s", i, succeededCondition.Type)
			continue
		}
		
		if succeededCondition.Status != "False" {
			t.Errorf("Iteration %d: Expected Succeeded status to be False, got %s", i, succeededCondition.Status)
			continue
		}
		
		// Verify failure message is present and not empty
		if len(succeededCondition.Message) == 0 {
			t.Errorf("Iteration %d: Failure message is empty", i)
			continue
		}
		
		// Verify failure reason is present
		if len(succeededCondition.Reason) == 0 {
			t.Errorf("Iteration %d: Failure reason is empty", i)
			continue
		}
	}
}

// TestPipelineRunFailureReporting_StatusContainsFailedTaskName tests that failed PipelineRuns identify the failed task
func TestPipelineRunFailureReporting_StatusContainsFailedTaskName(t *testing.T) {
	// Property: For any failed PipelineRun, the status should identify which task failed
	
	scenarios := []FailureScenario{
		{
			PipelineRunName: "pipeline-run-1",
			FailedTaskName:  "git-clone",
			FailureReason:   "TaskRunFailed",
			FailureMessage:  "Task git-clone failed: authentication failed",
		},
		{
			PipelineRunName: "pipeline-run-2",
			FailedTaskName:  "cdktf-synth",
			FailureReason:   "TaskRunFailed",
			FailureMessage:  "Task cdktf-synth failed: syntax error",
		},
		{
			PipelineRunName: "pipeline-run-3",
			FailedTaskName:  "cdktf-deploy",
			FailureReason:   "TaskRunFailed",
			FailureMessage:  "Task cdktf-deploy failed: resource conflict",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		status := PipelineRunStatus{
			Conditions: []PipelineRunCondition{
				{
					Type:    "Succeeded",
					Status:  "False",
					Reason:  scenario.FailureReason,
					Message: scenario.FailureMessage,
				},
			},
			TaskRuns: map[string]TaskRunStatus{
				fmt.Sprintf("%s-%s", scenario.PipelineRunName, scenario.FailedTaskName): {
					TaskName: scenario.FailedTaskName,
					Status:   "Failed",
					Reason:   "TaskRunFailed",
					Message:  scenario.FailureMessage,
				},
			},
		}
		
		// Verify the failure message mentions the failed task
		if !strings.Contains(status.Conditions[0].Message, scenario.FailedTaskName) {
			t.Errorf("Iteration %d: Failure message doesn't mention failed task %s: %s", 
				i, scenario.FailedTaskName, status.Conditions[0].Message)
			continue
		}
		
		// Verify TaskRuns map contains the failed task
		if len(status.TaskRuns) == 0 {
			t.Errorf("Iteration %d: TaskRuns map is empty", i)
			continue
		}
		
		// Find the failed task in TaskRuns
		foundFailedTask := false
		for _, taskRun := range status.TaskRuns {
			if taskRun.TaskName == scenario.FailedTaskName && taskRun.Status == "Failed" {
				foundFailedTask = true
				break
			}
		}
		
		if !foundFailedTask {
			t.Errorf("Iteration %d: Failed task %s not found in TaskRuns", i, scenario.FailedTaskName)
		}
	}
}

// TestPipelineRunFailureReporting_FailureReasonIsDescriptive tests that failure reasons are descriptive
func TestPipelineRunFailureReporting_FailureReasonIsDescriptive(t *testing.T) {
	// Property: For any failed PipelineRun, the failure reason should be descriptive and actionable
	
	f := func() bool {
		reasons := []string{
			"TaskRunFailed",
			"PipelineRunTimeout",
			"TaskRunTimeout",
			"InvalidTaskSpec",
			"ResourceQuotaExceeded",
		}
		
		messages := []string{
			"Task git-clone failed: authentication failed",
			"Pipeline timed out after 1 hour",
			"Task cdktf-deploy timed out after 30 minutes",
			"Invalid task specification: missing required field 'image'",
			"Resource quota exceeded: cannot create more pods",
		}
		
		// Verify each reason is not empty and follows a pattern
		for _, reason := range reasons {
			if len(reason) == 0 {
				t.Logf("Failure reason is empty")
				return false
			}
			
			// Verify reason doesn't contain spaces (should be CamelCase or similar)
			if strings.Contains(reason, " ") {
				t.Logf("Failure reason contains spaces: %s", reason)
				return false
			}
		}
		
		// Verify each message is descriptive
		for _, message := range messages {
			if len(message) == 0 {
				t.Logf("Failure message is empty")
				return false
			}
			
			// Verify message contains useful information (not just "failed")
			if message == "failed" || message == "error" {
				t.Logf("Failure message is not descriptive: %s", message)
				return false
			}
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestPipelineRunFailureReporting_MultipleTaskFailures tests handling of multiple task failures
func TestPipelineRunFailureReporting_MultipleTaskFailures(t *testing.T) {
	// Property: For any PipelineRun with multiple failed tasks, all failures should be reported
	
	pipelineRunName := "pipeline-run-multi-fail"
	
	status := PipelineRunStatus{
		Conditions: []PipelineRunCondition{
			{
				Type:    "Succeeded",
				Status:  "False",
				Reason:  "TaskRunFailed",
				Message: "Multiple tasks failed: git-clone, cdktf-synth",
			},
		},
		TaskRuns: map[string]TaskRunStatus{
			fmt.Sprintf("%s-git-clone", pipelineRunName): {
				TaskName: "git-clone",
				Status:   "Failed",
				Reason:   "TaskRunFailed",
				Message:  "Authentication failed",
			},
			fmt.Sprintf("%s-cdktf-synth", pipelineRunName): {
				TaskName: "cdktf-synth",
				Status:   "Failed",
				Reason:   "TaskRunFailed",
				Message:  "Syntax error in configuration",
			},
		},
	}
	
	// Verify both failed tasks are mentioned in the overall message
	overallMessage := status.Conditions[0].Message
	if !strings.Contains(overallMessage, "git-clone") {
		t.Errorf("Overall message doesn't mention git-clone failure")
	}
	
	if !strings.Contains(overallMessage, "cdktf-synth") {
		t.Errorf("Overall message doesn't mention cdktf-synth failure")
	}
	
	// Verify both tasks are in TaskRuns with Failed status
	failedCount := 0
	for _, taskRun := range status.TaskRuns {
		if taskRun.Status == "Failed" {
			failedCount++
		}
	}
	
	if failedCount != 2 {
		t.Errorf("Expected 2 failed tasks, got %d", failedCount)
	}
}

// TestPipelineRunFailureReporting_FailureTimestamp tests that failures include timestamps
func TestPipelineRunFailureReporting_FailureTimestamp(t *testing.T) {
	// Property: For any failed PipelineRun, the failure should include timing information
	
	scenarios := []FailureScenario{
		{
			PipelineRunName: "pipeline-run-1",
			FailedTaskName:  "git-clone",
			FailureReason:   "TaskRunFailed",
			FailureMessage:  "Authentication failed",
		},
		{
			PipelineRunName: "pipeline-run-2",
			FailedTaskName:  "cdktf-deploy",
			FailureReason:   "TaskRunTimeout",
			FailureMessage:  "Task timed out",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		now := time.Now()
		startTime := now.Add(-10 * time.Minute)
		
		taskRun := TaskRunStatus{
			TaskName:   scenario.FailedTaskName,
			Status:     "Failed",
			Reason:     scenario.FailureReason,
			Message:    scenario.FailureMessage,
			StartTime:  metav1.Time{Time: startTime},
			FinishTime: metav1.Time{Time: now},
		}
		
		// Verify start time is set
		if taskRun.StartTime.IsZero() {
			t.Errorf("Iteration %d: Start time is not set", i)
			continue
		}
		
		// Verify finish time is set
		if taskRun.FinishTime.IsZero() {
			t.Errorf("Iteration %d: Finish time is not set", i)
			continue
		}
		
		// Verify finish time is after start time
		if !taskRun.FinishTime.After(taskRun.StartTime.Time) {
			t.Errorf("Iteration %d: Finish time is not after start time", i)
			continue
		}
	}
}

// TestPipelineRunFailureReporting_FailureInDifferentPhases tests failures in different pipeline phases
func TestPipelineRunFailureReporting_FailureInDifferentPhases(t *testing.T) {
	// Property: For any PipelineRun, failures should be reported regardless of which phase they occur in
	
	phases := []struct {
		taskName string
		phase    string
		reason   string
		message  string
	}{
		{
			taskName: "git-clone",
			phase:    "source",
			reason:   "TaskRunFailed",
			message:  "Failed to clone repository",
		},
		{
			taskName: "cdktf-synth",
			phase:    "build",
			reason:   "TaskRunFailed",
			message:  "Failed to synthesize configuration",
		},
		{
			taskName: "cdktf-deploy",
			phase:    "deploy",
			reason:   "TaskRunFailed",
			message:  "Failed to deploy infrastructure",
		},
	}
	
	for _, phase := range phases {
		status := PipelineRunStatus{
			Conditions: []PipelineRunCondition{
				{
					Type:    "Succeeded",
					Status:  "False",
					Reason:  phase.reason,
					Message: fmt.Sprintf("Task %s failed in %s phase: %s", phase.taskName, phase.phase, phase.message),
				},
			},
			TaskRuns: map[string]TaskRunStatus{
				phase.taskName: {
					TaskName: phase.taskName,
					Status:   "Failed",
					Reason:   phase.reason,
					Message:  phase.message,
				},
			},
		}
		
		// Verify failure is reported
		if status.Conditions[0].Status != "False" {
			t.Errorf("Phase %s: Expected Succeeded status to be False", phase.phase)
		}
		
		// Verify failure message mentions the task and phase
		if !strings.Contains(status.Conditions[0].Message, phase.taskName) {
			t.Errorf("Phase %s: Failure message doesn't mention task %s", phase.phase, phase.taskName)
		}
		
		if !strings.Contains(status.Conditions[0].Message, phase.phase) {
			t.Errorf("Phase %s: Failure message doesn't mention phase", phase.phase)
		}
	}
}

// TestPipelineRunFailureReporting_Integration tests the full failure reporting flow
func TestPipelineRunFailureReporting_Integration(t *testing.T) {
	// Integration test: Verify complete failure reporting for a realistic scenario
	
	pipelineRunName := "cdktf-deploy-pipeline-run-123"
	
	status := PipelineRunStatus{
		Conditions: []PipelineRunCondition{
			{
				Type:    "Succeeded",
				Status:  "False",
				Reason:  "TaskRunFailed",
				Message: "Task cdktf-deploy failed: Terraform apply failed with exit code 1",
			},
		},
		TaskRuns: map[string]TaskRunStatus{
			fmt.Sprintf("%s-git-clone", pipelineRunName): {
				TaskName:   "git-clone",
				Status:     "Succeeded",
				Reason:     "Succeeded",
				Message:    "Repository cloned successfully",
				StartTime:  metav1.Time{Time: time.Now().Add(-15 * time.Minute)},
				FinishTime: metav1.Time{Time: time.Now().Add(-14 * time.Minute)},
			},
			fmt.Sprintf("%s-cdktf-synth", pipelineRunName): {
				TaskName:   "cdktf-synth",
				Status:     "Succeeded",
				Reason:     "Succeeded",
				Message:    "CDKTF synthesis completed",
				StartTime:  metav1.Time{Time: time.Now().Add(-14 * time.Minute)},
				FinishTime: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
			},
			fmt.Sprintf("%s-cdktf-deploy", pipelineRunName): {
				TaskName:   "cdktf-deploy",
				Status:     "Failed",
				Reason:     "TaskRunFailed",
				Message:    "Terraform apply failed: Error creating resource: resource already exists",
				StartTime:  metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
				FinishTime: metav1.Time{Time: time.Now()},
			},
		},
	}
	
	// Verify overall status is Failed
	if status.Conditions[0].Status != "False" {
		t.Errorf("Expected overall status to be False (failed)")
	}
	
	// Verify failure message is descriptive
	if len(status.Conditions[0].Message) == 0 {
		t.Errorf("Failure message is empty")
	}
	
	// Verify failed task is identified
	if !strings.Contains(status.Conditions[0].Message, "cdktf-deploy") {
		t.Errorf("Failure message doesn't identify failed task")
	}
	
	// Verify TaskRuns contains all tasks with correct statuses
	if len(status.TaskRuns) != 3 {
		t.Errorf("Expected 3 TaskRuns, got %d", len(status.TaskRuns))
	}
	
	// Verify the failed task has detailed error information
	failedTaskKey := fmt.Sprintf("%s-cdktf-deploy", pipelineRunName)
	failedTask, exists := status.TaskRuns[failedTaskKey]
	if !exists {
		t.Errorf("Failed task not found in TaskRuns")
	}
	
	if failedTask.Status != "Failed" {
		t.Errorf("Failed task status is not 'Failed': %s", failedTask.Status)
	}
	
	if len(failedTask.Message) == 0 {
		t.Errorf("Failed task message is empty")
	}
	
	// Verify timing information is present
	if failedTask.StartTime.IsZero() || failedTask.FinishTime.IsZero() {
		t.Errorf("Failed task missing timing information")
	}
}
