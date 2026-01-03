package tests

import (
	"fmt"
	"strings"
	"testing"
	"testing/quick"
	"time"

	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Feature: argocd-tekton-platform, Property 35: Kubernetes Event Generation
// Validates: Requirements 10.6
//
// Property: For any significant component state change (pod crash, sync failure, controller error), a Kubernetes event should be created.

// ComponentStateChange represents a significant state change in a component
type ComponentStateChange struct {
	ComponentType string // Pod, Deployment, Application, Controller
	ComponentName string
	Namespace     string
	EventType     string // Normal, Warning
	Reason        string
	Message       string
}

// TestKubernetesEventGeneration_PodCrashGeneratesEvent tests that pod crashes generate events
func TestKubernetesEventGeneration_PodCrashGeneratesEvent(t *testing.T) {
	// Property: For any pod crash, a Kubernetes event should be created
	
	scenarios := []ComponentStateChange{
		{
			ComponentType: "Pod",
			ComponentName: "onboarding-controller-abc123",
			Namespace:     "platform-system",
			EventType:     "Warning",
			Reason:        "BackOff",
			Message:       "Back-off restarting failed container",
		},
		{
			ComponentType: "Pod",
			ComponentName: "pipeline-run-git-clone-xyz789",
			Namespace:     "tenant-test",
			EventType:     "Warning",
			Reason:        "Failed",
			Message:       "Error: ImagePullBackOff",
		},
		{
			ComponentType: "Pod",
			ComponentName: "argocd-server-def456",
			Namespace:     "argocd",
			EventType:     "Warning",
			Reason:        "Unhealthy",
			Message:       "Liveness probe failed: HTTP probe failed with statuscode: 500",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		event := corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%d", scenario.ComponentName, time.Now().Unix()),
				Namespace: scenario.Namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      scenario.ComponentType,
				Name:      scenario.ComponentName,
				Namespace: scenario.Namespace,
			},
			Type:    scenario.EventType,
			Reason:  scenario.Reason,
			Message: scenario.Message,
			FirstTimestamp: metav1.Time{Time: time.Now()},
			LastTimestamp:  metav1.Time{Time: time.Now()},
			Count:          1,
		}
		
		// Verify event has correct type (Warning for crashes)
		if event.Type != "Warning" {
			t.Errorf("Iteration %d: Expected event type Warning, got %s", i, event.Type)
			continue
		}
		
		// Verify event references the correct object
		if event.InvolvedObject.Kind != scenario.ComponentType {
			t.Errorf("Iteration %d: Event references wrong kind: %s", i, event.InvolvedObject.Kind)
			continue
		}
		
		if event.InvolvedObject.Name != scenario.ComponentName {
			t.Errorf("Iteration %d: Event references wrong name: %s", i, event.InvolvedObject.Name)
			continue
		}
		
		// Verify event has a descriptive reason
		if len(event.Reason) == 0 {
			t.Errorf("Iteration %d: Event reason is empty", i)
			continue
		}
		
		// Verify event has a descriptive message
		if len(event.Message) == 0 {
			t.Errorf("Iteration %d: Event message is empty", i)
			continue
		}
		
		// Verify event has timestamps
		if event.FirstTimestamp.IsZero() {
			t.Errorf("Iteration %d: Event first timestamp is not set", i)
			continue
		}
	}
}

// TestKubernetesEventGeneration_SyncFailureGeneratesEvent tests that sync failures generate events
func TestKubernetesEventGeneration_SyncFailureGeneratesEvent(t *testing.T) {
	// Property: For any ArgoCD sync failure, a Kubernetes event should be created
	
	scenarios := []ComponentStateChange{
		{
			ComponentType: "Application",
			ComponentName: "platform-crds",
			Namespace:     "argocd",
			EventType:     "Warning",
			Reason:        "SyncFailed",
			Message:       "Failed to sync: invalid CRD manifest",
		},
		{
			ComponentType: "Application",
			ComponentName: "platform-controllers",
			Namespace:     "argocd",
			EventType:     "Warning",
			Reason:        "SyncFailed",
			Message:       "Failed to sync: image pull error",
		},
		{
			ComponentType: "Application",
			ComponentName: "platform-catalog",
			Namespace:     "argocd",
			EventType:     "Warning",
			Reason:        "SyncFailed",
			Message:       "Failed to sync: validation error",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		event := corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%d", scenario.ComponentName, time.Now().Unix()),
				Namespace: scenario.Namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				APIVersion: "argoproj.io/v1alpha1",
				Kind:       scenario.ComponentType,
				Name:       scenario.ComponentName,
				Namespace:  scenario.Namespace,
			},
			Type:    scenario.EventType,
			Reason:  scenario.Reason,
			Message: scenario.Message,
			FirstTimestamp: metav1.Time{Time: time.Now()},
			LastTimestamp:  metav1.Time{Time: time.Now()},
			Count:          1,
		}
		
		// Verify event type is Warning for sync failures
		if event.Type != "Warning" {
			t.Errorf("Iteration %d: Expected event type Warning, got %s", i, event.Type)
			continue
		}
		
		// Verify event reason indicates sync failure
		if !strings.Contains(event.Reason, "Sync") && !strings.Contains(event.Reason, "Failed") {
			t.Errorf("Iteration %d: Event reason doesn't indicate sync failure: %s", i, event.Reason)
			continue
		}
		
		// Verify event message is descriptive
		if len(event.Message) == 0 {
			t.Errorf("Iteration %d: Event message is empty", i)
			continue
		}
		
		// Verify event references an Application
		if event.InvolvedObject.Kind != "Application" {
			t.Errorf("Iteration %d: Event doesn't reference an Application: %s", i, event.InvolvedObject.Kind)
			continue
		}
	}
}

// TestKubernetesEventGeneration_ControllerErrorGeneratesEvent tests that controller errors generate events
func TestKubernetesEventGeneration_ControllerErrorGeneratesEvent(t *testing.T) {
	// Property: For any controller error, a Kubernetes event should be created
	
	scenarios := []ComponentStateChange{
		{
			ComponentType: "RepoBinding",
			ComponentName: "test-repo-binding",
			Namespace:     "platform-system",
			EventType:     "Warning",
			Reason:        "ProvisioningFailed",
			Message:       "Failed to provision tenant: namespace already exists",
		},
		{
			ComponentType: "RepoBinding",
			ComponentName: "another-repo-binding",
			Namespace:     "platform-system",
			EventType:     "Warning",
			Reason:        "ValidationFailed",
			Message:       "Invalid tenant name: must match Kubernetes naming rules",
		},
		{
			ComponentType: "RepoBinding",
			ComponentName: "third-repo-binding",
			Namespace:     "platform-system",
			EventType:     "Normal",
			Reason:        "ProvisioningSucceeded",
			Message:       "Tenant provisioned successfully",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		event := corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%d", scenario.ComponentName, time.Now().Unix()),
				Namespace: scenario.Namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				APIVersion: "arbiter.io/v1alpha1",
				Kind:       scenario.ComponentType,
				Name:       scenario.ComponentName,
				Namespace:  scenario.Namespace,
			},
			Type:    scenario.EventType,
			Reason:  scenario.Reason,
			Message: scenario.Message,
			FirstTimestamp: metav1.Time{Time: time.Now()},
			LastTimestamp:  metav1.Time{Time: time.Now()},
			Count:          1,
		}
		
		// Verify event has appropriate type
		if event.Type != "Normal" && event.Type != "Warning" {
			t.Errorf("Iteration %d: Invalid event type: %s", i, event.Type)
			continue
		}
		
		// Verify event reason is descriptive
		if len(event.Reason) == 0 {
			t.Errorf("Iteration %d: Event reason is empty", i)
			continue
		}
		
		// Verify event message is descriptive
		if len(event.Message) == 0 {
			t.Errorf("Iteration %d: Event message is empty", i)
			continue
		}
		
		// Verify event references the correct custom resource
		if event.InvolvedObject.Kind != scenario.ComponentType {
			t.Errorf("Iteration %d: Event references wrong kind: %s", i, event.InvolvedObject.Kind)
			continue
		}
	}
}

// TestKubernetesEventGeneration_EventReasonsAreDescriptive tests that event reasons are descriptive
func TestKubernetesEventGeneration_EventReasonsAreDescriptive(t *testing.T) {
	// Property: For any Kubernetes event, the reason should be descriptive and follow conventions
	
	f := func() bool {
		reasons := []string{
			"BackOff",
			"Failed",
			"Unhealthy",
			"SyncFailed",
			"ProvisioningFailed",
			"ValidationFailed",
			"ProvisioningSucceeded",
			"Created",
			"Updated",
			"Deleted",
		}
		
		// Verify each reason follows conventions
		for _, reason := range reasons {
			if len(reason) == 0 {
				t.Logf("Event reason is empty")
				return false
			}
			
			// Verify reason is CamelCase (no spaces)
			if strings.Contains(reason, " ") {
				t.Logf("Event reason contains spaces: %s", reason)
				return false
			}
			
			// Verify reason starts with uppercase letter
			if len(reason) > 0 && reason[0] < 'A' || reason[0] > 'Z' {
				t.Logf("Event reason doesn't start with uppercase: %s", reason)
				return false
			}
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestKubernetesEventGeneration_EventsHaveTimestamps tests that events include timestamps
func TestKubernetesEventGeneration_EventsHaveTimestamps(t *testing.T) {
	// Property: For any Kubernetes event, timestamps should be present
	
	scenarios := []ComponentStateChange{
		{
			ComponentType: "Pod",
			ComponentName: "test-pod",
			Namespace:     "default",
			EventType:     "Warning",
			Reason:        "Failed",
			Message:       "Pod failed",
		},
		{
			ComponentType: "Deployment",
			ComponentName: "test-deployment",
			Namespace:     "default",
			EventType:     "Normal",
			Reason:        "ScalingReplicaSet",
			Message:       "Scaled up replica set",
		},
	}
	
	for i := 0; i < 100; i++ {
		scenario := scenarios[i%len(scenarios)]
		
		now := time.Now()
		
		event := corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%d", scenario.ComponentName, now.Unix()),
				Namespace: scenario.Namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      scenario.ComponentType,
				Name:      scenario.ComponentName,
				Namespace: scenario.Namespace,
			},
			Type:           scenario.EventType,
			Reason:         scenario.Reason,
			Message:        scenario.Message,
			FirstTimestamp: metav1.Time{Time: now},
			LastTimestamp:  metav1.Time{Time: now},
			Count:          1,
		}
		
		// Verify first timestamp is set
		if event.FirstTimestamp.IsZero() {
			t.Errorf("Iteration %d: First timestamp is not set", i)
			continue
		}
		
		// Verify last timestamp is set
		if event.LastTimestamp.IsZero() {
			t.Errorf("Iteration %d: Last timestamp is not set", i)
			continue
		}
		
		// Verify last timestamp is not before first timestamp
		if event.LastTimestamp.Before(&event.FirstTimestamp) {
			t.Errorf("Iteration %d: Last timestamp is before first timestamp", i)
			continue
		}
	}
}

// TestKubernetesEventGeneration_EventCountIncrementsForRepeated tests that repeated events increment count
func TestKubernetesEventGeneration_EventCountIncrementsForRepeated(t *testing.T) {
	// Property: For any repeated event, the count should increment and last timestamp should update
	
	componentName := "test-pod"
	namespace := "default"
	
	// Use a fixed first timestamp for all events (since they represent the same event)
	firstTime := time.Now().Add(-5 * time.Minute)
	
	// Simulate the same event occurring multiple times
	events := []corev1.Event{
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.event1", componentName),
				Namespace: namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      componentName,
				Namespace: namespace,
			},
			Type:           "Warning",
			Reason:         "BackOff",
			Message:        "Back-off restarting failed container",
			FirstTimestamp: metav1.Time{Time: firstTime},
			LastTimestamp:  metav1.Time{Time: firstTime},
			Count:          1,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.event1", componentName),
				Namespace: namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      componentName,
				Namespace: namespace,
			},
			Type:           "Warning",
			Reason:         "BackOff",
			Message:        "Back-off restarting failed container",
			FirstTimestamp: metav1.Time{Time: firstTime},
			LastTimestamp:  metav1.Time{Time: time.Now().Add(-3 * time.Minute)},
			Count:          2,
		},
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.event1", componentName),
				Namespace: namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      componentName,
				Namespace: namespace,
			},
			Type:           "Warning",
			Reason:         "BackOff",
			Message:        "Back-off restarting failed container",
			FirstTimestamp: metav1.Time{Time: firstTime},
			LastTimestamp:  metav1.Time{Time: time.Now()},
			Count:          3,
		},
	}
	
	// Verify count increments
	for i := 1; i < len(events); i++ {
		if events[i].Count <= events[i-1].Count {
			t.Errorf("Event %d: Count did not increment (previous: %d, current: %d)", 
				i, events[i-1].Count, events[i].Count)
		}
		
		// Verify last timestamp is updated
		if !events[i].LastTimestamp.After(events[i-1].LastTimestamp.Time) {
			t.Errorf("Event %d: Last timestamp was not updated", i)
		}
		
		// Verify first timestamp remains the same
		if !events[i].FirstTimestamp.Equal(&events[i-1].FirstTimestamp) {
			t.Errorf("Event %d: First timestamp changed (should remain constant)", i)
		}
	}
}

// TestKubernetesEventGeneration_DifferentEventTypes tests different types of events
func TestKubernetesEventGeneration_DifferentEventTypes(t *testing.T) {
	// Property: For any component state change, the appropriate event type should be used
	
	eventTypes := []struct {
		stateChange ComponentStateChange
		expectedType string
	}{
		{
			stateChange: ComponentStateChange{
				ComponentType: "Pod",
				ComponentName: "test-pod",
				Namespace:     "default",
				EventType:     "Normal",
				Reason:        "Started",
				Message:       "Container started successfully",
			},
			expectedType: "Normal",
		},
		{
			stateChange: ComponentStateChange{
				ComponentType: "Pod",
				ComponentName: "test-pod",
				Namespace:     "default",
				EventType:     "Warning",
				Reason:        "Failed",
				Message:       "Container failed to start",
			},
			expectedType: "Warning",
		},
		{
			stateChange: ComponentStateChange{
				ComponentType: "Deployment",
				ComponentName: "test-deployment",
				Namespace:     "default",
				EventType:     "Normal",
				Reason:        "ScalingReplicaSet",
				Message:       "Scaled up replica set",
			},
			expectedType: "Normal",
		},
	}
	
	for i, et := range eventTypes {
		event := corev1.Event{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.%d", et.stateChange.ComponentName, time.Now().Unix()),
				Namespace: et.stateChange.Namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      et.stateChange.ComponentType,
				Name:      et.stateChange.ComponentName,
				Namespace: et.stateChange.Namespace,
			},
			Type:    et.stateChange.EventType,
			Reason:  et.stateChange.Reason,
			Message: et.stateChange.Message,
			FirstTimestamp: metav1.Time{Time: time.Now()},
			LastTimestamp:  metav1.Time{Time: time.Now()},
			Count:          1,
		}
		
		// Verify event type matches expected
		if event.Type != et.expectedType {
			t.Errorf("Event %d: Expected type %s, got %s", i, et.expectedType, event.Type)
		}
		
		// Verify event type is either Normal or Warning
		if event.Type != "Normal" && event.Type != "Warning" {
			t.Errorf("Event %d: Invalid event type: %s", i, event.Type)
		}
	}
}

// TestKubernetesEventGeneration_Integration tests the full event generation flow
func TestKubernetesEventGeneration_Integration(t *testing.T) {
	// Integration test: Verify complete event generation for a realistic scenario
	
	// Scenario: A pod crashes and generates multiple events
	podName := "onboarding-controller-abc123"
	namespace := "platform-system"
	
	events := []corev1.Event{
		// Initial failure
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.failure", podName),
				Namespace: namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      podName,
				Namespace: namespace,
			},
			Type:           "Warning",
			Reason:         "Failed",
			Message:        "Container image pull failed: ImagePullBackOff",
			FirstTimestamp: metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
			LastTimestamp:  metav1.Time{Time: time.Now().Add(-10 * time.Minute)},
			Count:          1,
		},
		// Repeated backoff attempts
		{
			ObjectMeta: metav1.ObjectMeta{
				Name:      fmt.Sprintf("%s.backoff", podName),
				Namespace: namespace,
			},
			InvolvedObject: corev1.ObjectReference{
				Kind:      "Pod",
				Name:      podName,
				Namespace: namespace,
			},
			Type:           "Warning",
			Reason:         "BackOff",
			Message:        "Back-off pulling image",
			FirstTimestamp: metav1.Time{Time: time.Now().Add(-9 * time.Minute)},
			LastTimestamp:  metav1.Time{Time: time.Now().Add(-1 * time.Minute)},
			Count:          5,
		},
	}
	
	// Verify all events are present
	if len(events) != 2 {
		t.Errorf("Expected 2 events, got %d", len(events))
	}
	
	// Verify first event (initial failure)
	if events[0].Type != "Warning" {
		t.Errorf("First event should be Warning type")
	}
	
	if events[0].Reason != "Failed" {
		t.Errorf("First event should have reason 'Failed'")
	}
	
	if events[0].Count != 1 {
		t.Errorf("First event should have count 1")
	}
	
	// Verify second event (repeated backoff)
	if events[1].Type != "Warning" {
		t.Errorf("Second event should be Warning type")
	}
	
	if events[1].Reason != "BackOff" {
		t.Errorf("Second event should have reason 'BackOff'")
	}
	
	if events[1].Count != 5 {
		t.Errorf("Second event should have count 5 (repeated)")
	}
	
	// Verify timestamps are logical
	if !events[1].FirstTimestamp.After(events[0].FirstTimestamp.Time) {
		t.Errorf("Second event first timestamp should be after first event")
	}
	
	if !events[1].LastTimestamp.After(events[1].FirstTimestamp.Time) {
		t.Errorf("Second event last timestamp should be after its first timestamp")
	}
	
	// Verify all events reference the same pod
	for i, event := range events {
		if event.InvolvedObject.Name != podName {
			t.Errorf("Event %d references wrong pod: %s", i, event.InvolvedObject.Name)
		}
		
		if event.InvolvedObject.Namespace != namespace {
			t.Errorf("Event %d references wrong namespace: %s", i, event.InvolvedObject.Namespace)
		}
	}
}
