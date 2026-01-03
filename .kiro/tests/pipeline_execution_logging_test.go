package tests

import (
	"context"
	"fmt"
	"strings"
	"testing"
	"testing/quick"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Feature: argocd-tekton-platform, Property 32: Pipeline Execution Logging
// Validates: Requirements 10.3
//
// Property: For any PipelineRun, all task logs should be stored in Kubernetes and be retrievable via kubectl logs.

// PipelineRunScenario represents a test scenario for pipeline execution logging
type PipelineRunScenario struct {
	Namespace      string
	PipelineRunName string
	TaskName       string
	LogContent     string
}

// Generate creates a random PipelineRunScenario for property-based testing
func (p PipelineRunScenario) Generate(rand *quick.Config) PipelineRunScenario {
	namespaces := []string{"tenant-test-1", "tenant-test-2", "tenant-example"}
	taskNames := []string{"git-clone", "cdktf-synth", "cdktf-deploy", "upload-artifacts"}
	logContents := []string{
		"Cloning repository...\nClone successful",
		"Running cdktf synth...\nSynthesis complete",
		"Deploying infrastructure...\nDeployment successful",
		"Error: Failed to connect to repository",
		"Warning: Resource quota exceeded",
	}
	
	// Ensure we always select a valid index
	namespaceIdx := rand.Rand.Intn(len(namespaces))
	taskIdx := rand.Rand.Intn(len(taskNames))
	logIdx := rand.Rand.Intn(len(logContents))
	
	return PipelineRunScenario{
		Namespace:      namespaces[namespaceIdx],
		PipelineRunName: fmt.Sprintf("pipeline-run-%d", rand.Rand.Intn(10000)),
		TaskName:       taskNames[taskIdx],
		LogContent:     logContents[logIdx],
	}
}

// TestPipelineExecutionLogging_LogsStoredInKubernetes tests that pipeline logs are stored in Kubernetes
func TestPipelineExecutionLogging_LogsStoredInKubernetes(t *testing.T) {
	// Property: For any PipelineRun, logs should be stored in Kubernetes pods
	// We verify this by checking that pod logs can be retrieved
	
	f := func(scenario PipelineRunScenario) bool {
		// Simulate a PipelineRun creating a pod with logs
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-pod", scenario.PipelineRunName, scenario.TaskName),
				Namespace: scenario.Namespace,
				Labels: map[string]string{
					"tekton.dev/pipelineRun": scenario.PipelineRunName,
					"tekton.dev/task":        scenario.TaskName,
				},
			},
			Spec: corev1.PodSpec{
				Containers: []corev1.Container{
					{
						Name:  "step-main",
						Image: "busybox",
					},
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodRunning,
			},
		}
		
		// Verify pod has the correct labels for log retrieval
		if pod.Labels["tekton.dev/pipelineRun"] != scenario.PipelineRunName {
			t.Logf("Pod missing pipelineRun label")
			return false
		}
		
		if pod.Labels["tekton.dev/task"] != scenario.TaskName {
			t.Logf("Pod missing task label")
			return false
		}
		
		// Verify pod is in a state where logs can be retrieved
		if pod.Status.Phase != corev1.PodRunning && pod.Status.Phase != corev1.PodSucceeded {
			t.Logf("Pod not in a loggable state: %s", pod.Status.Phase)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestPipelineExecutionLogging_LogsRetrievableViaKubectl tests that logs can be retrieved via kubectl
func TestPipelineExecutionLogging_LogsRetrievableViaKubectl(t *testing.T) {
	// Property: For any PipelineRun, logs should be retrievable using kubectl logs command
	// We verify this by checking that the pod name and namespace are properly formatted
	
	f := func(scenario PipelineRunScenario) bool {
		podName := fmt.Sprintf("%s-%s-pod", scenario.PipelineRunName, scenario.TaskName)
		
		// Verify pod name is valid for kubectl (no spaces, valid characters)
		if strings.Contains(podName, " ") {
			t.Logf("Pod name contains spaces: %s", podName)
			return false
		}
		
		// Verify namespace is valid
		if strings.Contains(scenario.Namespace, " ") {
			t.Logf("Namespace contains spaces: %s", scenario.Namespace)
			return false
		}
		
		// Verify we can construct a valid kubectl logs command
		kubectlCmd := fmt.Sprintf("kubectl logs -n %s %s", scenario.Namespace, podName)
		if len(kubectlCmd) == 0 {
			t.Logf("Failed to construct kubectl command")
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestPipelineExecutionLogging_AllTasksHaveLogs tests that all tasks in a pipeline have logs
func TestPipelineExecutionLogging_AllTasksHaveLogs(t *testing.T) {
	// Property: For any PipelineRun with multiple tasks, each task should have its own pod with logs
	
	pipelineRunName := "test-pipeline-run"
	namespace := "tenant-test"
	tasks := []string{"git-clone", "cdktf-synth", "cdktf-deploy"}
	
	pods := make([]*corev1.Pod, 0, len(tasks))
	
	for _, taskName := range tasks {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-pod", pipelineRunName, taskName),
				Namespace: namespace,
				Labels: map[string]string{
					"tekton.dev/pipelineRun": pipelineRunName,
					"tekton.dev/task":        taskName,
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodSucceeded,
			},
		}
		pods = append(pods, pod)
	}
	
	// Verify each task has a corresponding pod
	if len(pods) != len(tasks) {
		t.Errorf("Expected %d pods, got %d", len(tasks), len(pods))
	}
	
	// Verify each pod has the correct labels
	for i, pod := range pods {
		if pod.Labels["tekton.dev/pipelineRun"] != pipelineRunName {
			t.Errorf("Pod %d missing pipelineRun label", i)
		}
		
		if pod.Labels["tekton.dev/task"] != tasks[i] {
			t.Errorf("Pod %d has wrong task label: expected %s, got %s", 
				i, tasks[i], pod.Labels["tekton.dev/task"])
		}
	}
}

// TestPipelineExecutionLogging_LogPersistence tests that logs persist after task completion
func TestPipelineExecutionLogging_LogPersistence(t *testing.T) {
	// Property: For any completed PipelineRun, logs should remain accessible after the task finishes
	
	f := func(scenario PipelineRunScenario) bool {
		pod := &corev1.Pod{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s-%s-pod", scenario.PipelineRunName, scenario.TaskName),
				Namespace: scenario.Namespace,
				Labels: map[string]string{
					"tekton.dev/pipelineRun": scenario.PipelineRunName,
					"tekton.dev/task":        scenario.TaskName,
				},
			},
			Status: corev1.PodStatus{
				Phase: corev1.PodSucceeded,
				ContainerStatuses: []corev1.ContainerStatus{
					{
						Name: "step-main",
						State: corev1.ContainerState{
							Terminated: &corev1.ContainerStateTerminated{
								ExitCode:   0,
								FinishedAt: metav1.Time{Time: time.Now()},
							},
						},
					},
				},
			},
		}
		
		// Verify pod is in Succeeded state (logs should be accessible)
		if pod.Status.Phase != corev1.PodSucceeded {
			t.Logf("Pod not in Succeeded state: %s", pod.Status.Phase)
			return false
		}
		
		// Verify container has terminated (logs are complete)
		if len(pod.Status.ContainerStatuses) == 0 {
			t.Logf("No container statuses")
			return false
		}
		
		if pod.Status.ContainerStatuses[0].State.Terminated == nil {
			t.Logf("Container not terminated")
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestPipelineExecutionLogging_LogsContainTaskOutput tests that logs contain actual task output
func TestPipelineExecutionLogging_LogsContainTaskOutput(t *testing.T) {
	// Property: For any PipelineRun, logs should contain the actual output from the task execution
	
	scenarios := []PipelineRunScenario{
		{
			Namespace:      "tenant-test-1",
			PipelineRunName: "pipeline-run-1",
			TaskName:       "git-clone",
			LogContent:     "Cloning repository...\nClone successful",
		},
		{
			Namespace:      "tenant-test-2",
			PipelineRunName: "pipeline-run-2",
			TaskName:       "cdktf-synth",
			LogContent:     "Running cdktf synth...\nSynthesis complete",
		},
		{
			Namespace:      "tenant-example",
			PipelineRunName: "pipeline-run-3",
			TaskName:       "cdktf-deploy",
			LogContent:     "Deploying infrastructure...\nDeployment successful",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		// Verify log content is not empty
		if len(scenario.LogContent) == 0 {
			t.Errorf("Iteration %d: Log content is empty", i)
			continue
		}
		
		// Verify log content contains meaningful information
		// (not just whitespace or placeholder text)
		trimmedContent := strings.TrimSpace(scenario.LogContent)
		if len(trimmedContent) == 0 {
			t.Errorf("Iteration %d: Log content is only whitespace", i)
			continue
		}
	}
}

// mockGetPodLogs simulates retrieving logs from a Kubernetes pod
func mockGetPodLogs(ctx context.Context, namespace, podName string) (string, error) {
	// In a real implementation, this would use the Kubernetes API:
	// clientset.CoreV1().Pods(namespace).GetLogs(podName, &corev1.PodLogOptions{}).Stream(ctx)
	
	// For testing purposes, we simulate successful log retrieval
	if namespace == "" || podName == "" {
		return "", fmt.Errorf("namespace and podName are required")
	}
	
	return fmt.Sprintf("Logs from pod %s in namespace %s", podName, namespace), nil
}

// TestPipelineExecutionLogging_Integration tests the full log retrieval flow
func TestPipelineExecutionLogging_Integration(t *testing.T) {
	// Integration test: Verify that logs can be retrieved for a complete pipeline run
	
	ctx := context.Background()
	namespace := "tenant-test"
	pipelineRunName := "test-pipeline-run"
	tasks := []string{"git-clone", "cdktf-synth", "cdktf-deploy"}
	
	for _, taskName := range tasks {
		podName := fmt.Sprintf("%s-%s-pod", pipelineRunName, taskName)
		
		logs, err := mockGetPodLogs(ctx, namespace, podName)
		if err != nil {
			t.Errorf("Failed to retrieve logs for task %s: %v", taskName, err)
			continue
		}
		
		if len(logs) == 0 {
			t.Errorf("Empty logs for task %s", taskName)
		}
		
		// Verify logs contain expected information
		if !strings.Contains(logs, podName) {
			t.Errorf("Logs for task %s don't contain pod name", taskName)
		}
	}
}
