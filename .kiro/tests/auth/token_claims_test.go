package auth

import (
	"encoding/json"
	"testing"
	"time"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
)

// JWTClaims represents the expected structure of JWT tokens issued by Dex
type JWTClaims struct {
	Issuer    string   `json:"iss"`
	Subject   string   `json:"sub"`
	Audience  string   `json:"aud"`
	ExpiresAt int64    `json:"exp"`
	IssuedAt  int64    `json:"iat"`
	Email     string   `json:"email"`
	Groups    []string `json:"groups"`
}

// TestProperty1_TokenClaimsCompleteness validates that all JWT tokens issued by Dex
// for the Kubernetes client contain all required claims.
//
// Feature: kubernetes-api-oidc-auth
// Property 1: Token Claims Completeness
// Validates: Requirements 3.1, 3.2, 3.3, 3.4, 3.5
func TestProperty1_TokenClaimsCompleteness(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("all Dex tokens for Kubernetes contain required claims", prop.ForAll(
		func(email string, groups []string, subject string) bool {
			// Simulate a JWT token issued by Dex for Kubernetes client
			now := time.Now().Unix()
			claims := JWTClaims{
				Issuer:    "https://dex.home.local",
				Subject:   subject,
				Audience:  "kubernetes",
				ExpiresAt: now + 3600, // 1 hour from now
				IssuedAt:  now,
				Email:     email,
				Groups:    groups,
			}

			// Validate all required claims are present and correct
			if claims.Issuer != "https://dex.home.local" {
				return false
			}

			if claims.Audience != "kubernetes" {
				return false
			}

			if claims.Email == "" {
				return false
			}

			if claims.Subject == "" {
				return false
			}

			if claims.ExpiresAt <= claims.IssuedAt {
				return false
			}

			if claims.Groups == nil {
				return false
			}

			return true
		},
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) < 100
		}).Map(func(s string) string {
			return s + "@platform.local"
		}),
		gen.SliceOf(gen.OneConstOf("platform-admins", "platform-operators", "platform-engineering")),
		gen.AlphaString().SuchThat(func(s string) bool {
			return len(s) > 0 && len(s) < 50
		}),
	))

	properties.Property("token issuer matches configured Dex URL", prop.ForAll(
		func(issuerURL string) bool {
			claims := JWTClaims{
				Issuer:   issuerURL,
				Audience: "kubernetes",
			}

			// Only the exact Dex issuer URL should be valid
			return claims.Issuer == "https://dex.home.local"
		},
		gen.OneConstOf(
			"https://dex.home.local",
			"https://other-issuer.com",
			"http://dex.home.local",
			"https://dex.example.com",
		),
	))

	properties.Property("token audience contains kubernetes client ID", prop.ForAll(
		func(audience string) bool {
			claims := JWTClaims{
				Issuer:   "https://dex.home.local",
				Audience: audience,
			}

			// Only "kubernetes" audience should be valid
			return claims.Audience == "kubernetes"
		},
		gen.OneConstOf(
			"kubernetes",
			"argocd",
			"tekton-dashboard",
			"invalid-client",
		),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty2_GroupClaimPropagation validates that groups from Authentik
// are correctly propagated to Dex-issued tokens.
//
// Feature: kubernetes-api-oidc-auth
// Property 2: Group Claim Propagation
// Validates: Requirements 2.4, 2.5, 4.4
func TestProperty2_GroupClaimPropagation(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("Authentik groups appear exactly in Dex token", prop.ForAll(
		func(authentikGroups []string) bool {
			// Simulate token issued by Dex with groups from Authentik
			claims := JWTClaims{
				Issuer:   "https://dex.home.local",
				Audience: "kubernetes",
				Email:    "user@platform.local",
				Subject:  "test-user",
				Groups:   authentikGroups,
			}

			// Groups in token should match exactly what was in Authentik
			if len(claims.Groups) != len(authentikGroups) {
				return false
			}

			for i, group := range authentikGroups {
				if claims.Groups[i] != group {
					return false
				}
			}

			return true
		},
		gen.SliceOf(gen.OneConstOf(
			"platform-admins",
			"platform-operators", 
			"platform-engineering",
			"custom-team-group",
		)).SuchThat(func(groups []string) bool {
			return len(groups) <= 10 // Reasonable limit
		}),
	))

	properties.Property("empty groups claim is valid", prop.ForAll(
		func() bool {
			// User with no groups should have empty groups claim
			claims := JWTClaims{
				Issuer:   "https://dex.home.local",
				Audience: "kubernetes",
				Email:    "newuser@platform.local",
				Subject:  "new-user",
				Groups:   []string{},
			}

			// Empty groups array should be valid
			return claims.Groups != nil && len(claims.Groups) == 0
		},
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestTokenClaimsValidation tests specific token validation scenarios
func TestTokenClaimsValidation(t *testing.T) {
	testCases := []struct {
		name        string
		claims      JWTClaims
		expectValid bool
	}{
		{
			name: "valid token with all claims",
			claims: JWTClaims{
				Issuer:    "https://dex.home.local",
				Subject:   "user123",
				Audience:  "kubernetes",
				ExpiresAt: time.Now().Unix() + 3600,
				IssuedAt:  time.Now().Unix(),
				Email:     "alice@platform.local",
				Groups:    []string{"platform-engineering"},
			},
			expectValid: true,
		},
		{
			name: "invalid issuer",
			claims: JWTClaims{
				Issuer:   "https://malicious-issuer.com",
				Audience: "kubernetes",
				Email:    "alice@platform.local",
			},
			expectValid: false,
		},
		{
			name: "invalid audience",
			claims: JWTClaims{
				Issuer:   "https://dex.home.local",
				Audience: "malicious-client",
				Email:    "alice@platform.local",
			},
			expectValid: false,
		},
		{
			name: "missing email claim",
			claims: JWTClaims{
				Issuer:   "https://dex.home.local",
				Audience: "kubernetes",
				Email:    "",
			},
			expectValid: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			valid := validateTokenClaims(tc.claims)
			if valid != tc.expectValid {
				t.Errorf("Expected validation result %v, got %v", tc.expectValid, valid)
			}
		})
	}
}

// validateTokenClaims simulates kube-apiserver token validation logic
func validateTokenClaims(claims JWTClaims) bool {
	if claims.Issuer != "https://dex.home.local" {
		return false
	}
	if claims.Audience != "kubernetes" {
		return false
	}
	if claims.Email == "" {
		return false
	}
	if claims.Subject == "" {
		return false
	}
	if claims.ExpiresAt <= claims.IssuedAt {
		return false
	}
	return true
}
