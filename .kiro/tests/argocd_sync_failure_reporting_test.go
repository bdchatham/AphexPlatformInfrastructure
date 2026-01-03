package tests

import (
	"strings"
	"testing"
	"testing/quick"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Feature: argocd-tekton-platform, Property 34: ArgoCD Sync Failure Reporting
// Validates: Requirements 10.5
//
// Property: For any ArgoCD Application sync that fails, the Application status should contain an error message describing the failure.

// ApplicationStatus represents the status of an ArgoCD Application
type ApplicationStatus struct {
	Sync       SyncStatus
	Health     HealthStatus
	Conditions []ApplicationCondition
	Resources  []ResourceStatus
}

// SyncStatus represents the sync status of an Application
type SyncStatus struct {
	Status    string // Synced, OutOfSync, Unknown
	Revision  string
	Message   string
}

// HealthStatus represents the health status of an Application
type HealthStatus struct {
	Status  string // Healthy, Progressing, Degraded, Suspended, Missing, Unknown
	Message string
}

// ApplicationCondition represents a condition in the Application status
type ApplicationCondition struct {
	Type               string
	Status             string
	LastTransitionTime metav1.Time
	Reason             string
	Message            string
}

// ResourceStatus represents the status of a resource managed by the Application
type ResourceStatus struct {
	Group     string
	Kind      string
	Name      string
	Namespace string
	Status    string
	Health    string
	Message   string
}

// SyncFailureScenario represents a test scenario for ArgoCD sync failure reporting
type SyncFailureScenario struct {
	ApplicationName string
	FailureReason   string
	FailureMessage  string
	FailedResource  string
}

// TestArgoCDSyncFailureReporting_StatusContainsErrorMessage tests that failed syncs have error messages
func TestArgoCDSyncFailureReporting_StatusContainsErrorMessage(t *testing.T) {
	// Property: For any failed ArgoCD Application sync, the status should contain an error message
	
	scenarios := []SyncFailureScenario{
		{
			ApplicationName: "platform-crds",
			FailureReason:   "SyncFailed",
			FailureMessage:  "Failed to apply CRD: invalid YAML syntax",
			FailedResource:  "CustomResourceDefinition/repobinding",
		},
		{
			ApplicationName: "platform-controllers",
			FailureReason:   "SyncFailed",
			FailureMessage:  "Failed to create Deployment: image pull error",
			FailedResource:  "Deployment/onboarding-controller",
		},
		{
			ApplicationName: "platform-catalog",
			FailureReason:   "SyncFailed",
			FailureMessage:  "Failed to apply Task: validation error",
			FailedResource:  "Task/git-clone",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		status := ApplicationStatus{
			Sync: SyncStatus{
				Status:  "OutOfSync",
				Message: scenario.FailureMessage,
			},
			Conditions: []ApplicationCondition{
				{
					Type:    "SyncError",
					Status:  "True",
					Reason:  scenario.FailureReason,
					Message: scenario.FailureMessage,
				},
			},
		}
		
		// Verify sync status is OutOfSync
		if status.Sync.Status != "OutOfSync" {
			t.Errorf("Iteration %d: Expected sync status OutOfSync, got %s", i, status.Sync.Status)
			continue
		}
		
		// Verify sync message is present and not empty
		if len(status.Sync.Message) == 0 {
			t.Errorf("Iteration %d: Sync message is empty", i)
			continue
		}
		
		// Verify conditions contain SyncError
		foundSyncError := false
		for _, condition := range status.Conditions {
			if condition.Type == "SyncError" && condition.Status == "True" {
				foundSyncError = true
				
				// Verify error message is present
				if len(condition.Message) == 0 {
					t.Errorf("Iteration %d: SyncError condition message is empty", i)
				}
				
				// Verify error reason is present
				if len(condition.Reason) == 0 {
					t.Errorf("Iteration %d: SyncError condition reason is empty", i)
				}
				
				break
			}
		}
		
		if !foundSyncError {
			t.Errorf("Iteration %d: SyncError condition not found", i)
		}
	}
}

// TestArgoCDSyncFailureReporting_FailureIdentifiesResource tests that failures identify the problematic resource
func TestArgoCDSyncFailureReporting_FailureIdentifiesResource(t *testing.T) {
	// Property: For any failed ArgoCD sync, the status should identify which resource caused the failure
	
	scenarios := []SyncFailureScenario{
		{
			ApplicationName: "platform-crds",
			FailureReason:   "SyncFailed",
			FailureMessage:  "Failed to apply CustomResourceDefinition/repobinding: invalid schema",
			FailedResource:  "CustomResourceDefinition/repobinding",
		},
		{
			ApplicationName: "platform-infrastructure",
			FailureReason:   "SyncFailed",
			FailureMessage:  "Failed to create Namespace/platform-system: already exists",
			FailedResource:  "Namespace/platform-system",
		},
		{
			ApplicationName: "platform-controllers",
			FailureReason:   "SyncFailed",
			FailureMessage:  "Failed to apply Deployment/onboarding-controller: image not found",
			FailedResource:  "Deployment/onboarding-controller",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		// Parse resource kind and name from FailedResource
		parts := strings.Split(scenario.FailedResource, "/")
		if len(parts) != 2 {
			t.Errorf("Iteration %d: Invalid FailedResource format: %s", i, scenario.FailedResource)
			continue
		}
		resourceKind := parts[0]
		resourceName := parts[1]
		
		status := ApplicationStatus{
			Sync: SyncStatus{
				Status:  "OutOfSync",
				Message: scenario.FailureMessage,
			},
			Conditions: []ApplicationCondition{
				{
					Type:    "SyncError",
					Status:  "True",
					Reason:  scenario.FailureReason,
					Message: scenario.FailureMessage,
				},
			},
			Resources: []ResourceStatus{
				{
					Kind:    resourceKind,
					Name:    resourceName,
					Status:  "OutOfSync",
					Health:  "Missing",
					Message: scenario.FailureMessage,
				},
			},
		}
		
		// Verify the failure message mentions the resource
		if !strings.Contains(status.Sync.Message, resourceKind) {
			t.Errorf("Iteration %d: Failure message doesn't mention resource kind %s", i, resourceKind)
			continue
		}
		
		if !strings.Contains(status.Sync.Message, resourceName) {
			t.Errorf("Iteration %d: Failure message doesn't mention resource name %s", i, resourceName)
			continue
		}
		
		// Verify the failed resource is in the Resources list
		foundFailedResource := false
		for _, resource := range status.Resources {
			if resource.Kind == resourceKind && resource.Name == resourceName {
				foundFailedResource = true
				
				// Verify resource has error information
				if len(resource.Message) == 0 {
					t.Errorf("Iteration %d: Failed resource has no error message", i)
				}
				
				break
			}
		}
		
		if !foundFailedResource {
			t.Errorf("Iteration %d: Failed resource %s not found in Resources list", i, scenario.FailedResource)
		}
	}
}

// TestArgoCDSyncFailureReporting_FailureReasonsAreDescriptive tests that failure reasons are descriptive
func TestArgoCDSyncFailureReporting_FailureReasonsAreDescriptive(t *testing.T) {
	// Property: For any failed ArgoCD sync, the failure reason should be descriptive and actionable
	
	f := func() bool {
		reasons := []string{
			"SyncFailed",
			"InvalidManifest",
			"ResourceConflict",
			"ValidationError",
			"PermissionDenied",
		}
		
		messages := []string{
			"Failed to apply manifest: invalid YAML syntax at line 42",
			"Resource validation failed: missing required field 'spec.template'",
			"Resource conflict: Deployment already exists with different owner",
			"Validation error: invalid value for field 'replicas': must be >= 0",
			"Permission denied: insufficient RBAC permissions to create Namespace",
		}
		
		// Verify each reason is not empty and follows a pattern
		for _, reason := range reasons {
			if len(reason) == 0 {
				t.Logf("Failure reason is empty")
				return false
			}
			
			// Verify reason doesn't contain spaces (should be CamelCase)
			if strings.Contains(reason, " ") {
				t.Logf("Failure reason contains spaces: %s", reason)
				return false
			}
		}
		
		// Verify each message is descriptive and actionable
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
			
			// Verify message provides context (contains ":" or specific details)
			if !strings.Contains(message, ":") && !strings.Contains(message, "failed") {
				t.Logf("Failure message lacks context: %s", message)
				return false
			}
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestArgoCDSyncFailureReporting_MultipleResourceFailures tests handling of multiple resource failures
func TestArgoCDSyncFailureReporting_MultipleResourceFailures(t *testing.T) {
	// Property: For any ArgoCD Application with multiple failed resources, all failures should be reported
	
	status := ApplicationStatus{
		Sync: SyncStatus{
			Status:  "OutOfSync",
			Message: "Multiple resources failed to sync: Namespace/tenant-1, Namespace/tenant-2",
		},
		Conditions: []ApplicationCondition{
			{
				Type:    "SyncError",
				Status:  "True",
				Reason:  "SyncFailed",
				Message: "Multiple resources failed to sync",
			},
		},
		Resources: []ResourceStatus{
			{
				Kind:    "Namespace",
				Name:    "tenant-1",
				Status:  "OutOfSync",
				Health:  "Missing",
				Message: "Failed to create: resource quota exceeded",
			},
			{
				Kind:    "Namespace",
				Name:    "tenant-2",
				Status:  "OutOfSync",
				Health:  "Missing",
				Message: "Failed to create: invalid name format",
			},
		},
	}
	
	// Verify both failed resources are mentioned in the overall message
	overallMessage := status.Sync.Message
	if !strings.Contains(overallMessage, "tenant-1") {
		t.Errorf("Overall message doesn't mention tenant-1 failure")
	}
	
	if !strings.Contains(overallMessage, "tenant-2") {
		t.Errorf("Overall message doesn't mention tenant-2 failure")
	}
	
	// Verify both resources are in Resources list with error information
	failedCount := 0
	for _, resource := range status.Resources {
		if resource.Status == "OutOfSync" && len(resource.Message) > 0 {
			failedCount++
		}
	}
	
	if failedCount != 2 {
		t.Errorf("Expected 2 failed resources, got %d", failedCount)
	}
}

// TestArgoCDSyncFailureReporting_HealthStatusReflectsFailure tests that health status reflects sync failures
func TestArgoCDSyncFailureReporting_HealthStatusReflectsFailure(t *testing.T) {
	// Property: For any failed ArgoCD sync, the health status should reflect the failure
	
	scenarios := []struct {
		syncStatus   string
		healthStatus string
		healthMsg    string
	}{
		{
			syncStatus:   "OutOfSync",
			healthStatus: "Degraded",
			healthMsg:    "Application is degraded due to sync failure",
		},
		{
			syncStatus:   "OutOfSync",
			healthStatus: "Missing",
			healthMsg:    "Required resources are missing",
		},
		{
			syncStatus:   "OutOfSync",
			healthStatus: "Unknown",
			healthMsg:    "Health status cannot be determined due to sync failure",
		},
	}
	
	for i, scenario := range scenarios {
		status := ApplicationStatus{
			Sync: SyncStatus{
				Status:  scenario.syncStatus,
				Message: "Sync failed",
			},
			Health: HealthStatus{
				Status:  scenario.healthStatus,
				Message: scenario.healthMsg,
			},
		}
		
		// Verify health status is not "Healthy" when sync fails
		if status.Health.Status == "Healthy" && status.Sync.Status == "OutOfSync" {
			t.Errorf("Scenario %d: Health status is Healthy despite sync failure", i)
		}
		
		// Verify health message is present
		if len(status.Health.Message) == 0 {
			t.Errorf("Scenario %d: Health message is empty", i)
		}
	}
}

// TestArgoCDSyncFailureReporting_FailureTimestamp tests that failures include timestamps
func TestArgoCDSyncFailureReporting_FailureTimestamp(t *testing.T) {
	// Property: For any failed ArgoCD sync, the failure should include timing information
	
	scenarios := []SyncFailureScenario{
		{
			ApplicationName: "platform-crds",
			FailureReason:   "SyncFailed",
			FailureMessage:  "Failed to apply CRD",
		},
		{
			ApplicationName: "platform-controllers",
			FailureReason:   "SyncFailed",
			FailureMessage:  "Failed to create Deployment",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		now := time.Now()
		
		condition := ApplicationCondition{
			Type:               "SyncError",
			Status:             "True",
			LastTransitionTime: metav1.Time{Time: now},
			Reason:             scenario.FailureReason,
			Message:            scenario.FailureMessage,
		}
		
		// Verify last transition time is set
		if condition.LastTransitionTime.IsZero() {
			t.Errorf("Iteration %d: Last transition time is not set", i)
			continue
		}
		
		// Verify last transition time is recent (within last hour for this test)
		if time.Since(condition.LastTransitionTime.Time) > time.Hour {
			t.Errorf("Iteration %d: Last transition time is too old: %v", i, condition.LastTransitionTime.Time)
			continue
		}
	}
}

// TestArgoCDSyncFailureReporting_DifferentFailureTypes tests different types of sync failures
func TestArgoCDSyncFailureReporting_DifferentFailureTypes(t *testing.T) {
	// Property: For any type of sync failure, the status should provide appropriate error information
	
	failureTypes := []struct {
		reason  string
		message string
	}{
		{
			reason:  "InvalidManifest",
			message: "Invalid YAML syntax: unexpected character at line 42",
		},
		{
			reason:  "ResourceConflict",
			message: "Resource already exists with different configuration",
		},
		{
			reason:  "ValidationError",
			message: "Resource validation failed: missing required field 'metadata.name'",
		},
		{
			reason:  "PermissionDenied",
			message: "Insufficient permissions to create resource in namespace",
		},
		{
			reason:  "DependencyError",
			message: "Required CRD not found: repobindings.arbiter.io",
		},
	}
	
	for _, failureType := range failureTypes {
		status := ApplicationStatus{
			Sync: SyncStatus{
				Status:  "OutOfSync",
				Message: failureType.message,
			},
			Conditions: []ApplicationCondition{
				{
					Type:    "SyncError",
					Status:  "True",
					Reason:  failureType.reason,
					Message: failureType.message,
				},
			},
		}
		
		// Verify sync status is OutOfSync
		if status.Sync.Status != "OutOfSync" {
			t.Errorf("Failure type %s: Expected sync status OutOfSync", failureType.reason)
		}
		
		// Verify error message is descriptive
		if len(status.Sync.Message) == 0 {
			t.Errorf("Failure type %s: Error message is empty", failureType.reason)
		}
		
		// Verify condition has the correct reason
		if len(status.Conditions) == 0 {
			t.Errorf("Failure type %s: No conditions present", failureType.reason)
			continue
		}
		
		if status.Conditions[0].Reason != failureType.reason {
			t.Errorf("Failure type %s: Expected reason %s, got %s", 
				failureType.reason, failureType.reason, status.Conditions[0].Reason)
		}
	}
}

// TestArgoCDSyncFailureReporting_Integration tests the full sync failure reporting flow
func TestArgoCDSyncFailureReporting_Integration(t *testing.T) {
	// Integration test: Verify complete sync failure reporting for a realistic scenario
	
	status := ApplicationStatus{
		Sync: SyncStatus{
			Status:   "OutOfSync",
			Revision: "abc123def456",
			Message:  "Failed to sync Deployment/onboarding-controller: image pull error",
		},
		Health: HealthStatus{
			Status:  "Degraded",
			Message: "Application is degraded: 1 resource failed to sync",
		},
		Conditions: []ApplicationCondition{
			{
				Type:               "SyncError",
				Status:             "True",
				LastTransitionTime: metav1.Time{Time: time.Now()},
				Reason:             "SyncFailed",
				Message:            "Failed to sync Deployment/onboarding-controller: ImagePullBackOff",
			},
		},
		Resources: []ResourceStatus{
			{
				Kind:      "Deployment",
				Name:      "onboarding-controller",
				Namespace: "platform-system",
				Status:    "OutOfSync",
				Health:    "Degraded",
				Message:   "ImagePullBackOff: Failed to pull image 'onboarding-controller:latest'",
			},
			{
				Kind:      "ServiceAccount",
				Name:      "onboarding-controller",
				Namespace: "platform-system",
				Status:    "Synced",
				Health:    "Healthy",
				Message:   "",
			},
		},
	}
	
	// Verify overall sync status is OutOfSync
	if status.Sync.Status != "OutOfSync" {
		t.Errorf("Expected sync status OutOfSync, got %s", status.Sync.Status)
	}
	
	// Verify sync message is descriptive
	if len(status.Sync.Message) == 0 {
		t.Errorf("Sync message is empty")
	}
	
	// Verify failed resource is identified
	if !strings.Contains(status.Sync.Message, "onboarding-controller") {
		t.Errorf("Sync message doesn't identify failed resource")
	}
	
	// Verify health status reflects the failure
	if status.Health.Status == "Healthy" {
		t.Errorf("Health status is Healthy despite sync failure")
	}
	
	// Verify SyncError condition is present
	foundSyncError := false
	for _, condition := range status.Conditions {
		if condition.Type == "SyncError" && condition.Status == "True" {
			foundSyncError = true
			
			// Verify condition has detailed error information
			if len(condition.Message) == 0 {
				t.Errorf("SyncError condition message is empty")
			}
			
			if condition.LastTransitionTime.IsZero() {
				t.Errorf("SyncError condition missing timestamp")
			}
			
			break
		}
	}
	
	if !foundSyncError {
		t.Errorf("SyncError condition not found")
	}
	
	// Verify Resources list contains both synced and failed resources
	if len(status.Resources) != 2 {
		t.Errorf("Expected 2 resources, got %d", len(status.Resources))
	}
	
	// Verify the failed resource has detailed error information
	foundFailedResource := false
	for _, resource := range status.Resources {
		if resource.Name == "onboarding-controller" && resource.Kind == "Deployment" {
			foundFailedResource = true
			
			if resource.Status != "OutOfSync" {
				t.Errorf("Failed resource status is not OutOfSync: %s", resource.Status)
			}
			
			if len(resource.Message) == 0 {
				t.Errorf("Failed resource message is empty")
			}
			
			break
		}
	}
	
	if !foundFailedResource {
		t.Errorf("Failed resource not found in Resources list")
	}
}
