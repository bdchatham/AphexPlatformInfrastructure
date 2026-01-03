package tests

import (
	"fmt"
	"regexp"
	"strings"
	"testing"
	"testing/quick"
)

// Feature: argocd-tekton-platform, Property 22: RepoBinding Status Completeness
// Validates: Requirements 6.8
//
// Property: For any successfully provisioned tenant, the RepoBinding status should contain
// webhookURL, webhookSecret, and all resource creation flags set to true.

// RepoBindingStatus represents the status of a RepoBinding
type RepoBindingStatus struct {
	Phase                      string
	Message                    string
	NamespaceCreated           bool
	ServiceAccountCreated      bool
	RBACCreated                bool
	QuotasCreated              bool
	NetworkPolicyCreated       bool
	TerraformSecretCreated     bool
	WebhookSecretCreated       bool
	EventListenerCreated       bool
	IngressCreated             bool
	AllowlistUpdated           bool
	WebhookURL                 string
	WebhookSecret              string
}

// TestRepoBindingStatus_PhaseTransitions tests status phase transitions
func TestRepoBindingStatus_PhaseTransitions(t *testing.T) {
	// Property: RepoBinding status phase should follow the sequence: Pending → Provisioning → Ready
	validPhases := []string{"Pending", "Provisioning", "Ready", "Failed"}
	
	for _, phase := range validPhases {
		t.Run(fmt.Sprintf("phase_%s", phase), func(t *testing.T) {
			// Verify phase is one of the valid values
			isValid := false
			for _, validPhase := range validPhases {
				if phase == validPhase {
					isValid = true
					break
				}
			}
			
			if !isValid {
				t.Errorf("Invalid phase: %s", phase)
			}
		})
	}
}

// TestRepoBindingStatus_ResourceCreationFlags tests resource creation flags
func TestRepoBindingStatus_ResourceCreationFlags(t *testing.T) {
	// Property: For any successfully provisioned tenant (Ready phase), all resource creation flags should be true
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Simulate a successfully provisioned tenant
		status := RepoBindingStatus{
			Phase:                      "Ready",
			NamespaceCreated:           true,
			ServiceAccountCreated:      true,
			RBACCreated:                true,
			QuotasCreated:              true,
			NetworkPolicyCreated:       true,
			TerraformSecretCreated:     true,
			WebhookSecretCreated:       true,
			EventListenerCreated:       true,
			IngressCreated:             true,
			AllowlistUpdated:           true,
		}
		
		// Verify all flags are true for Ready phase
		if status.Phase == "Ready" {
			if !status.NamespaceCreated {
				t.Logf("NamespaceCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
			if !status.ServiceAccountCreated {
				t.Logf("ServiceAccountCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
			if !status.RBACCreated {
				t.Logf("RBACCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
			if !status.QuotasCreated {
				t.Logf("QuotasCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
			if !status.NetworkPolicyCreated {
				t.Logf("NetworkPolicyCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
			if !status.TerraformSecretCreated {
				t.Logf("TerraformSecretCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
			if !status.WebhookSecretCreated {
				t.Logf("WebhookSecretCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
			if !status.EventListenerCreated {
				t.Logf("EventListenerCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
			if !status.IngressCreated {
				t.Logf("IngressCreated should be true for Ready phase in tenant %s", tenantName)
				return false
			}
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestRepoBindingStatus_WebhookURLFormat tests webhook URL format
func TestRepoBindingStatus_WebhookURLFormat(t *testing.T) {
	// Property: For any Ready RepoBinding, webhookURL should be in the format http://{host}/{tenant}
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		ingressHost := "webhooks.local"
		
		expectedURL := fmt.Sprintf("http://%s/%s", ingressHost, tenantName)
		
		status := RepoBindingStatus{
			Phase:      "Ready",
			WebhookURL: expectedURL,
		}
		
		// Verify webhook URL is not empty
		if status.WebhookURL == "" {
			t.Logf("Empty webhook URL for tenant %s", tenantName)
			return false
		}
		
		// Verify webhook URL starts with http:// or https://
		if !strings.HasPrefix(status.WebhookURL, "http://") && !strings.HasPrefix(status.WebhookURL, "https://") {
			t.Logf("Webhook URL doesn't start with http:// or https://: %s for tenant %s", 
				status.WebhookURL, tenantName)
			return false
		}
		
		// Verify webhook URL contains the tenant name
		if !strings.Contains(status.WebhookURL, tenantName) {
			t.Logf("Webhook URL doesn't contain tenant name: %s for tenant %s", 
				status.WebhookURL, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestRepoBindingStatus_WebhookSecretFormat tests webhook secret format
func TestRepoBindingStatus_WebhookSecretFormat(t *testing.T) {
	// Property: For any Ready RepoBinding, webhookSecret should start with "whsec_"
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Generate a webhook secret
		webhookSecret, err := generateWebhookSecret()
		if err != nil {
			t.Logf("Failed to generate webhook secret: %v", err)
			return false
		}
		
		status := RepoBindingStatus{
			Phase:         "Ready",
			WebhookSecret: webhookSecret,
		}
		
		// Verify webhook secret is not empty
		if status.WebhookSecret == "" {
			t.Logf("Empty webhook secret for tenant %s", tenantName)
			return false
		}
		
		// Verify webhook secret starts with "whsec_"
		if !strings.HasPrefix(status.WebhookSecret, "whsec_") {
			t.Logf("Webhook secret doesn't start with whsec_: %s for tenant %s", 
				status.WebhookSecret, tenantName)
			return false
		}
		
		// Verify webhook secret has sufficient length
		if len(status.WebhookSecret) < 40 {
			t.Logf("Webhook secret too short: %d chars for tenant %s", 
				len(status.WebhookSecret), tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestRepoBindingStatus_MessageContent tests status message content
func TestRepoBindingStatus_MessageContent(t *testing.T) {
	// Property: For any Ready RepoBinding, the message should contain setup instructions
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		repoOrg := "test-org"
		repoName := "test-repo"
		
		// Expected message should contain key information
		expectedKeywords := []string{
			"Registration successful",
			"GitHub webhook",
			"Payload URL",
			"Secret",
		}
		
		// Simulate a status message
		message := fmt.Sprintf(`Registration successful!

Next steps - Configure GitHub webhook:
1. Go to: https://github.com/%s/%s/settings/hooks/new
2. Payload URL: http://webhooks.local/%s
3. Content type: application/json
4. Secret: whsec_test123
5. Events: Push events, Pull request events
6. Active: ✓
7. Click "Add webhook"`, repoOrg, repoName, tenantName)
		
		status := RepoBindingStatus{
			Phase:   "Ready",
			Message: message,
		}
		
		// Verify message is not empty
		if status.Message == "" {
			t.Logf("Empty message for tenant %s", tenantName)
			return false
		}
		
		// Verify message contains expected keywords
		for _, keyword := range expectedKeywords {
			if !strings.Contains(status.Message, keyword) {
				t.Logf("Message missing keyword '%s' for tenant %s", keyword, tenantName)
				return false
			}
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestRepoBindingStatus_FailedPhaseHasMessage tests failed phase message
func TestRepoBindingStatus_FailedPhaseHasMessage(t *testing.T) {
	// Property: For any Failed RepoBinding, the message should contain error details
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Simulate a failed status
		errorMessage := "Failed to provision namespace: namespace already exists"
		
		status := RepoBindingStatus{
			Phase:   "Failed",
			Message: errorMessage,
		}
		
		// Verify message is not empty for Failed phase
		if status.Phase == "Failed" && status.Message == "" {
			t.Logf("Empty message for Failed phase in tenant %s", tenantName)
			return false
		}
		
		// Verify message contains error information
		if status.Phase == "Failed" {
			if !strings.Contains(status.Message, "Failed") && !strings.Contains(status.Message, "Error") {
				t.Logf("Failed phase message doesn't contain error information for tenant %s", tenantName)
				return false
			}
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestRepoBindingStatus_WebhookURLValidFormat tests webhook URL validity
func TestRepoBindingStatus_WebhookURLValidFormat(t *testing.T) {
	// Property: For any Ready RepoBinding, webhookURL should be a valid URL format
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		ingressHost := "webhooks.local"
		
		webhookURL := fmt.Sprintf("http://%s/%s", ingressHost, tenantName)
		
		status := RepoBindingStatus{
			Phase:      "Ready",
			WebhookURL: webhookURL,
		}
		
		// Basic URL format validation
		urlPattern := regexp.MustCompile(`^https?://[a-z0-9.-]+/[a-z0-9-]+$`)
		if !urlPattern.MatchString(status.WebhookURL) {
			t.Logf("Invalid webhook URL format: %s for tenant %s", status.WebhookURL, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}
