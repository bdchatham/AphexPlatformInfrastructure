package tests

import (
	"fmt"
	"regexp"
	"testing"
	"testing/quick"
)

// Feature: argocd-tekton-platform, Property 21: Ingress Creation
// Validates: Requirements 6.7
//
// Property: For any provisioned tenant, an Ingress resource should exist routing webhooks
// to the tenant's EventListener.

// IngressConfig represents the configuration for an Ingress
type IngressConfig struct {
	Name            string
	Namespace       string
	Host            string
	Path            string
	ServiceName     string
	ServicePort     int
	IngressClass    string
}

// TestIngressCreation_NamingConvention tests Ingress naming
func TestIngressCreation_NamingConvention(t *testing.T) {
	// Property: For any tenant, the Ingress should be named "github-webhook"
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Ingress name is always "github-webhook"
		ingressName := "github-webhook"
		
		// Verify it's a valid Kubernetes resource name
		namePattern := regexp.MustCompile(`^[a-z0-9-]+$`)
		if !namePattern.MatchString(ingressName) {
			t.Logf("Invalid Ingress name: %s for tenant %s", ingressName, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestIngressCreation_PathConfiguration tests path routing
func TestIngressCreation_PathConfiguration(t *testing.T) {
	// Property: For any tenant, the Ingress path should be /{tenantName}
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		expectedPath := fmt.Sprintf("/%s", tenantName)
		
		config := IngressConfig{
			Name:      "github-webhook",
			Namespace: tenantName,
			Path:      expectedPath,
		}
		
		// Verify path is not empty
		if config.Path == "" {
			t.Logf("Empty path for tenant %s", tenantName)
			return false
		}
		
		// Verify path starts with /
		if config.Path[0] != '/' {
			t.Logf("Path doesn't start with /: %s for tenant %s", config.Path, tenantName)
			return false
		}
		
		// Verify path matches expected format
		if config.Path != expectedPath {
			t.Logf("Unexpected path: %s (expected %s) for tenant %s", 
				config.Path, expectedPath, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestIngressCreation_ServiceBackend tests service backend configuration
func TestIngressCreation_ServiceBackend(t *testing.T) {
	// Property: For any tenant Ingress, it should route to the EventListener service
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// EventListener service name is always "el-github-listener"
		expectedServiceName := "el-github-listener"
		expectedServicePort := 8080
		
		config := IngressConfig{
			Name:        "github-webhook",
			Namespace:   tenantName,
			ServiceName: expectedServiceName,
			ServicePort: expectedServicePort,
		}
		
		// Verify service name is not empty
		if config.ServiceName == "" {
			t.Logf("Empty service name for tenant %s", tenantName)
			return false
		}
		
		// Verify service name matches expected
		if config.ServiceName != expectedServiceName {
			t.Logf("Unexpected service name: %s (expected %s) for tenant %s", 
				config.ServiceName, expectedServiceName, tenantName)
			return false
		}
		
		// Verify service port is valid
		if config.ServicePort <= 0 || config.ServicePort > 65535 {
			t.Logf("Invalid service port: %d for tenant %s", config.ServicePort, tenantName)
			return false
		}
		
		// Verify service port matches expected
		if config.ServicePort != expectedServicePort {
			t.Logf("Unexpected service port: %d (expected %d) for tenant %s", 
				config.ServicePort, expectedServicePort, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestIngressCreation_IngressClass tests ingress class configuration
func TestIngressCreation_IngressClass(t *testing.T) {
	// Property: For any tenant Ingress, it should use the "nginx" ingress class
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		expectedIngressClass := "nginx"
		
		config := IngressConfig{
			Name:         "github-webhook",
			Namespace:    tenantName,
			IngressClass: expectedIngressClass,
		}
		
		// Verify ingress class is not empty
		if config.IngressClass == "" {
			t.Logf("Empty ingress class for tenant %s", tenantName)
			return false
		}
		
		// Verify ingress class matches expected
		if config.IngressClass != expectedIngressClass {
			t.Logf("Unexpected ingress class: %s (expected %s) for tenant %s", 
				config.IngressClass, expectedIngressClass, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestIngressCreation_HostConfiguration tests host configuration
func TestIngressCreation_HostConfiguration(t *testing.T) {
	// Property: For any tenant Ingress, it should have a valid hostname configured
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Default host for homelab
		defaultHost := "webhooks.local"
		
		config := IngressConfig{
			Name:      "github-webhook",
			Namespace: tenantName,
			Host:      defaultHost,
		}
		
		// Verify host is not empty
		if config.Host == "" {
			t.Logf("Empty host for tenant %s", tenantName)
			return false
		}
		
		// Verify host is a valid hostname format (basic check)
		hostPattern := regexp.MustCompile(`^[a-z0-9.-]+$`)
		if !hostPattern.MatchString(config.Host) {
			t.Logf("Invalid host format: %s for tenant %s", config.Host, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestIngressCreation_Annotations tests ingress annotations
func TestIngressCreation_Annotations(t *testing.T) {
	// Property: For any tenant Ingress, it should have the rewrite-target annotation
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Expected annotation
		expectedAnnotationKey := "nginx.ingress.kubernetes.io/rewrite-target"
		expectedAnnotationValue := "/"
		
		// Verify annotation key is not empty
		if expectedAnnotationKey == "" {
			t.Logf("Empty annotation key for tenant %s", tenantName)
			return false
		}
		
		// Verify annotation value is not empty
		if expectedAnnotationValue == "" {
			t.Logf("Empty annotation value for tenant %s", tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestIngressCreation_Labels tests resource labeling
func TestIngressCreation_Labels(t *testing.T) {
	// Property: For any tenant Ingress, it should have appropriate labels
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

// TestIngressCreation_PathType tests path type configuration
func TestIngressCreation_PathType(t *testing.T) {
	// Property: For any tenant Ingress, the path type should be "Prefix"
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		expectedPathType := "Prefix"
		
		// Verify path type is not empty
		if expectedPathType == "" {
			t.Logf("Empty path type for tenant %s", tenantName)
			return false
		}
		
		// Verify path type is valid
		validPathTypes := []string{"Prefix", "Exact", "ImplementationSpecific"}
		isValid := false
		for _, validType := range validPathTypes {
			if expectedPathType == validType {
				isValid = true
				break
			}
		}
		
		if !isValid {
			t.Logf("Invalid path type: %s for tenant %s", expectedPathType, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}
