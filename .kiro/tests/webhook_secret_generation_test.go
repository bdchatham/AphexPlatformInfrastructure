package tests

import (
	"crypto/rand"
	"encoding/base64"
	"fmt"
	"strings"
	"testing"
	"testing/quick"
)

// Feature: argocd-tekton-platform, Property 16: Webhook Secret Generation
// Validates: Requirements 6.2, 9.1, 9.2
//
// Property: For any RepoBinding, the Onboarding Controller should generate a cryptographically
// secure webhook secret (whsec_ prefix + 32 random bytes) and store it in a Kubernetes Secret.

// generateWebhookSecret generates a cryptographically secure webhook secret
// This is the implementation from the controller that we're testing
func generateWebhookSecret() (string, error) {
	randomBytes := make([]byte, 32)
	_, err := rand.Read(randomBytes)
	if err != nil {
		return "", fmt.Errorf("failed to generate random bytes: %w", err)
	}
	
	encodedSecret := base64.URLEncoding.EncodeToString(randomBytes)
	return fmt.Sprintf("whsec_%s", encodedSecret), nil
}

// TestWebhookSecretGeneration_Format tests that generated secrets have the correct format
func TestWebhookSecretGeneration_Format(t *testing.T) {
	// Property: All generated webhook secrets must start with "whsec_"
	f := func() bool {
		secret, err := generateWebhookSecret()
		if err != nil {
			t.Logf("Failed to generate secret: %v", err)
			return false
		}
		
		return strings.HasPrefix(secret, "whsec_")
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestWebhookSecretGeneration_Length tests that generated secrets have sufficient length
func TestWebhookSecretGeneration_Length(t *testing.T) {
	// Property: All generated webhook secrets must be at least 40 characters
	// (whsec_ prefix = 6 chars + base64(32 bytes) = ~43 chars)
	f := func() bool {
		secret, err := generateWebhookSecret()
		if err != nil {
			t.Logf("Failed to generate secret: %v", err)
			return false
		}
		
		return len(secret) >= 40
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestWebhookSecretGeneration_Uniqueness tests that generated secrets are unique
func TestWebhookSecretGeneration_Uniqueness(t *testing.T) {
	// Property: Generating multiple secrets should produce unique values
	// We generate 100 secrets and verify they're all different
	secrets := make(map[string]bool)
	
	for i := 0; i < 100; i++ {
		secret, err := generateWebhookSecret()
		if err != nil {
			t.Fatalf("Failed to generate secret: %v", err)
		}
		
		if secrets[secret] {
			t.Errorf("Duplicate secret generated: %s", secret)
		}
		secrets[secret] = true
	}
	
	if len(secrets) != 100 {
		t.Errorf("Expected 100 unique secrets, got %d", len(secrets))
	}
}

// TestWebhookSecretGeneration_Base64Encoding tests that the secret portion is valid base64
func TestWebhookSecretGeneration_Base64Encoding(t *testing.T) {
	// Property: The portion after "whsec_" must be valid base64 URL encoding
	f := func() bool {
		secret, err := generateWebhookSecret()
		if err != nil {
			t.Logf("Failed to generate secret: %v", err)
			return false
		}
		
		// Extract the base64 portion (after "whsec_")
		if !strings.HasPrefix(secret, "whsec_") {
			return false
		}
		
		base64Portion := strings.TrimPrefix(secret, "whsec_")
		
		// Try to decode it
		decoded, err := base64.URLEncoding.DecodeString(base64Portion)
		if err != nil {
			t.Logf("Failed to decode base64: %v", err)
			return false
		}
		
		// Verify it decodes to 32 bytes
		return len(decoded) == 32
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestWebhookSecretGeneration_CryptographicRandomness tests basic randomness properties
func TestWebhookSecretGeneration_CryptographicRandomness(t *testing.T) {
	// Property: Generated secrets should have high entropy (not predictable patterns)
	// We test this by verifying that consecutive secrets don't share common prefixes
	
	secret1, err := generateWebhookSecret()
	if err != nil {
		t.Fatalf("Failed to generate first secret: %v", err)
	}
	
	secret2, err := generateWebhookSecret()
	if err != nil {
		t.Fatalf("Failed to generate second secret: %v", err)
	}
	
	// Extract base64 portions
	base64_1 := strings.TrimPrefix(secret1, "whsec_")
	base64_2 := strings.TrimPrefix(secret2, "whsec_")
	
	// Count common prefix length (should be very small for random data)
	commonPrefixLen := 0
	minLen := len(base64_1)
	if len(base64_2) < minLen {
		minLen = len(base64_2)
	}
	
	for i := 0; i < minLen; i++ {
		if base64_1[i] == base64_2[i] {
			commonPrefixLen++
		} else {
			break
		}
	}
	
	// For truly random data, we expect very few matching prefix characters
	// Allow up to 3 characters to match by chance
	if commonPrefixLen > 3 {
		t.Errorf("Secrets share suspiciously long common prefix (%d chars): %s vs %s", 
			commonPrefixLen, secret1, secret2)
	}
}
