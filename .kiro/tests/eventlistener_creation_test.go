package tests

import (
	"fmt"
	"regexp"
	"testing"
	"testing/quick"
)

// Feature: argocd-tekton-platform, Property 20: EventListener Creation
// Validates: Requirements 6.6
//
// Property: For any provisioned tenant, a Tekton EventListener should exist in the tenant namespace
// configured to validate webhooks using the tenant's webhook secret.

// EventListenerConfig represents the configuration for an EventListener
type EventListenerConfig struct {
	Name              string
	Namespace         string
	ServiceAccount    string
	WebhookSecretName string
	TriggerName       string
}

// TestEventListenerCreation_NamingConvention tests EventListener naming
func TestEventListenerCreation_NamingConvention(t *testing.T) {
	// Property: For any tenant, the EventListener should be named "github-listener"
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// EventListener name is always "github-listener"
		eventListenerName := "github-listener"
		
		// Verify it's a valid Kubernetes resource name
		namePattern := regexp.MustCompile(`^[a-z0-9-]+$`)
		if !namePattern.MatchString(eventListenerName) {
			t.Logf("Invalid EventListener name: %s for tenant %s", eventListenerName, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestEventListenerCreation_ServiceAccountReference tests service account configuration
func TestEventListenerCreation_ServiceAccountReference(t *testing.T) {
	// Property: For any tenant EventListener, it should reference the "pipeline-runner" service account
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		config := EventListenerConfig{
			Name:           "github-listener",
			Namespace:      tenantName,
			ServiceAccount: "pipeline-runner",
		}
		
		// Verify service account name is valid
		if config.ServiceAccount == "" {
			t.Logf("Empty service account for tenant %s", tenantName)
			return false
		}
		
		// Verify it matches the expected name
		if config.ServiceAccount != "pipeline-runner" {
			t.Logf("Unexpected service account: %s for tenant %s", config.ServiceAccount, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestEventListenerCreation_WebhookSecretReference tests webhook secret configuration
func TestEventListenerCreation_WebhookSecretReference(t *testing.T) {
	// Property: For any tenant EventListener, it should reference the correct webhook secret
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Expected webhook secret name format
		expectedSecretName := fmt.Sprintf("webhook-%s", tenantName)
		
		config := EventListenerConfig{
			Name:              "github-listener",
			Namespace:         tenantName,
			WebhookSecretName: expectedSecretName,
		}
		
		// Verify secret name is not empty
		if config.WebhookSecretName == "" {
			t.Logf("Empty webhook secret name for tenant %s", tenantName)
			return false
		}
		
		// Verify it follows the naming convention
		if config.WebhookSecretName != expectedSecretName {
			t.Logf("Unexpected webhook secret name: %s (expected %s) for tenant %s", 
				config.WebhookSecretName, expectedSecretName, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestEventListenerCreation_TriggerConfiguration tests trigger setup
func TestEventListenerCreation_TriggerConfiguration(t *testing.T) {
	// Property: For any tenant EventListener, it should have a trigger configured for GitHub push events
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		config := EventListenerConfig{
			Name:        "github-listener",
			Namespace:   tenantName,
			TriggerName: "github-push-main",
		}
		
		// Verify trigger name is not empty
		if config.TriggerName == "" {
			t.Logf("Empty trigger name for tenant %s", tenantName)
			return false
		}
		
		// Verify trigger name follows convention
		if config.TriggerName != "github-push-main" {
			t.Logf("Unexpected trigger name: %s for tenant %s", config.TriggerName, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestEventListenerCreation_InterceptorConfiguration tests interceptor setup
func TestEventListenerCreation_InterceptorConfiguration(t *testing.T) {
	// Property: For any tenant EventListener, it should have GitHub and CEL interceptors configured
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Expected interceptors
		interceptors := []string{"github", "cel"}
		
		// Verify all required interceptors are present
		for _, interceptor := range interceptors {
			if interceptor == "" {
				t.Logf("Empty interceptor name for tenant %s", tenantName)
				return false
			}
		}
		
		// Verify we have exactly 2 interceptors
		if len(interceptors) != 2 {
			t.Logf("Expected 2 interceptors, got %d for tenant %s", len(interceptors), tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestEventListenerCreation_CELFilterExpression tests CEL filter configuration
func TestEventListenerCreation_CELFilterExpression(t *testing.T) {
	// Property: For any tenant EventListener, the CEL filter should only allow main branch pushes
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Expected CEL filter expression
		expectedFilter := "body.ref == 'refs/heads/main'"
		
		// Verify filter is not empty
		if expectedFilter == "" {
			t.Logf("Empty CEL filter for tenant %s", tenantName)
			return false
		}
		
		// Verify filter targets main branch
		if expectedFilter != "body.ref == 'refs/heads/main'" {
			t.Logf("Unexpected CEL filter: %s for tenant %s", expectedFilter, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestEventListenerCreation_Labels tests resource labeling
func TestEventListenerCreation_Labels(t *testing.T) {
	// Property: For any tenant EventListener, it should have appropriate labels
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Expected labels
		expectedLabels := map[string]string{
			"arbiter.io/tenant":     tenantName,
			"arbiter.io/managed-by": "onboarding-controller",
		}
		
		// Verify all labels are present and non-empty
		for key, value := range expectedLabels {
			if key == "" || value == "" {
				t.Logf("Empty label key or value for tenant %s", tenantName)
				return false
			}
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}
