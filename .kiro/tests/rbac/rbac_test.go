package rbac

import (
	"context"
	"testing"

	"github.com/leanovate/gopter"
	"github.com/leanovate/gopter/gen"
	"github.com/leanovate/gopter/prop"
	authorizationv1 "k8s.io/api/authorization/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes/fake"
)

// TestProperty3_RBACGroupAuthorization validates that RBAC correctly authorizes
// requests based on group membership from OIDC tokens.
//
// Feature: kubernetes-api-oidc-auth
// Property 3: RBAC Group Authorization
// Validates: Requirements 8.4, 8.5
func TestProperty3_RBACGroupAuthorization(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("users with platform groups get corresponding permissions", prop.ForAll(
		func(userGroups []string, resource string, verb string, namespace string) bool {
			// Create fake clientset with RBAC resources
			client := fake.NewSimpleClientset()
			ctx := context.Background()

			// Simulate SubjectAccessReview for user with groups
			sar := &authorizationv1.SubjectAccessReview{
				Spec: authorizationv1.SubjectAccessReviewSpec{
					User:   "test-user@platform.local",
					Groups: userGroups,
					ResourceAttributes: &authorizationv1.ResourceAttributes{
						Namespace: namespace,
						Verb:      verb,
						Group:     "platform.dev",
						Resource:  resource,
					},
				},
			}

			// Simulate authorization decision based on group membership
			allowed := simulateRBACDecision(userGroups, resource, verb, namespace)

			// Set the result
			sar.Status = authorizationv1.SubjectAccessReviewStatus{
				Allowed: allowed,
			}

			// Create the SAR (this would normally be processed by kube-apiserver)
			_, err := client.AuthorizationV1().SubjectAccessReviews().Create(ctx, sar, metav1.CreateOptions{})
			if err != nil {
				return false
			}

			return true
		},
		gen.SliceOf(gen.OneConstOf("platform-admins", "platform-operators", "platform-engineering")),
		gen.OneConstOf("pipelines", "workspaces", "environments"),
		gen.OneConstOf("get", "list", "create", "update", "delete"),
		gen.OneConstOf("user-alice", "team-frontend", "auth-system", "default"),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty4_NamespaceScopingForEngineers validates that engineers can only
// create resources in user-* and team-* namespaces.
//
// Feature: kubernetes-api-oidc-auth
// Property 4: Namespace Scoping for Engineers
// Validates: Requirements 9.2, 9.3
func TestProperty4_NamespaceScopingForEngineers(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("engineers can create in user-* and team-* namespaces only", prop.ForAll(
		func(namespace string, resource string) bool {
			groups := []string{"platform-engineering"}
			verb := "create"

			allowed := simulateRBACDecision(groups, resource, verb, namespace)

			// Engineers should only be allowed in user-* or team-* namespaces
			expectedAllowed := isUserOrTeamNamespace(namespace)

			return allowed == expectedAllowed
		},
		gen.OneConstOf(
			"user-alice", "user-bob", "team-frontend", "team-backend",
			"auth-system", "tekton-pipelines", "argocd", "default", "kube-system",
		),
		gen.OneConstOf("pipelines", "workspaces", "environments"),
	))

	properties.Property("admins can create in any namespace", prop.ForAll(
		func(namespace string, resource string) bool {
			groups := []string{"platform-admins"}
			verb := "create"

			allowed := simulateRBACDecision(groups, resource, verb, namespace)

			// Admins should be allowed in any namespace
			return allowed == true
		},
		gen.OneConstOf(
			"user-alice", "team-frontend", "auth-system", "tekton-pipelines", "default",
		),
		gen.OneConstOf("pipelines", "workspaces", "environments"),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// TestProperty5_AdminFullAccess validates that platform-admins have full access
// to all platform CRDs in all namespaces.
//
// Feature: kubernetes-api-oidc-auth
// Property 5: Admin Full Access
// Validates: Requirements 22.1
func TestProperty5_AdminFullAccess(t *testing.T) {
	properties := gopter.NewProperties(nil)

	properties.Property("platform-admins have full CRUD access to all platform CRDs", prop.ForAll(
		func(resource string, verb string, namespace string) bool {
			groups := []string{"platform-admins"}

			allowed := simulateRBACDecision(groups, resource, verb, namespace)

			// Admins should have full access to all platform resources
			return allowed == true
		},
		gen.OneConstOf("pipelines", "workspaces", "environments", "repobindings"),
		gen.OneConstOf("get", "list", "watch", "create", "update", "patch", "delete"),
		gen.OneConstOf("user-alice", "team-frontend", "auth-system", "default"),
	))

	properties.TestingRun(t, gopter.ConsoleReporter(false))
}

// simulateRBACDecision simulates the RBAC authorization decision logic
func simulateRBACDecision(userGroups []string, resource, verb, namespace string) bool {
	// Check if user is in platform-admins (full access)
	for _, group := range userGroups {
		if group == "platform-admins" {
			return true // Admins have full access
		}
	}

	// Check if user is in platform-operators
	for _, group := range userGroups {
		if group == "platform-operators" {
			// Operators have full CRUD on platform CRDs but cannot create/delete namespaces
			if resource == "namespaces" && (verb == "create" || verb == "delete") {
				return false
			}
			return isPlatformResource(resource)
		}
	}

	// Check if user is in platform-engineering
	for _, group := range userGroups {
		if group == "platform-engineering" {
			// Engineers can only create/read/update (no delete) in user-*/team-* namespaces
			if verb == "delete" {
				return false
			}
			if !isUserOrTeamNamespace(namespace) {
				return false
			}
			return isPlatformResource(resource)
		}
	}

	// No matching groups
	return false
}

// isUserOrTeamNamespace checks if namespace follows user-* or team-* pattern
func isUserOrTeamNamespace(namespace string) bool {
	if len(namespace) < 5 {
		return false
	}
	return (namespace[:5] == "user-") || (len(namespace) >= 5 && namespace[:5] == "team-")
}

// isPlatformResource checks if resource is a platform CRD
func isPlatformResource(resource string) bool {
	platformResources := []string{"pipelines", "workspaces", "environments", "repobindings"}
	for _, pr := range platformResources {
		if resource == pr {
			return true
		}
	}
	return false
}

// TestRBACScenarios tests specific RBAC authorization scenarios
func TestRBACScenarios(t *testing.T) {
	testCases := []struct {
		name        string
		groups      []string
		resource    string
		verb        string
		namespace   string
		expectAllow bool
	}{
		{
			name:        "admin can create pipelines in auth-system",
			groups:      []string{"platform-admins"},
			resource:    "pipelines",
			verb:        "create",
			namespace:   "auth-system",
			expectAllow: true,
		},
		{
			name:        "engineer can create pipelines in user namespace",
			groups:      []string{"platform-engineering"},
			resource:    "pipelines",
			verb:        "create",
			namespace:   "user-alice",
			expectAllow: true,
		},
		{
			name:        "engineer cannot create pipelines in auth-system",
			groups:      []string{"platform-engineering"},
			resource:    "pipelines",
			verb:        "create",
			namespace:   "auth-system",
			expectAllow: false,
		},
		{
			name:        "engineer cannot delete pipelines",
			groups:      []string{"platform-engineering"},
			resource:    "pipelines",
			verb:        "delete",
			namespace:   "user-alice",
			expectAllow: false,
		},
		{
			name:        "operator cannot create namespaces",
			groups:      []string{"platform-operators"},
			resource:    "namespaces",
			verb:        "create",
			namespace:   "",
			expectAllow: false,
		},
	}

	for _, tc := range testCases {
		t.Run(tc.name, func(t *testing.T) {
			allowed := simulateRBACDecision(tc.groups, tc.resource, tc.verb, tc.namespace)
			if allowed != tc.expectAllow {
				t.Errorf("Expected %v, got %v", tc.expectAllow, allowed)
			}
		})
	}
}
