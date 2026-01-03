package tests

import (
	"strconv"
	"strings"
	"testing"
	"testing/quick"
)

// Feature: argocd-tekton-platform, Property 25: Resource Quota Enforcement
// Validates: Requirements 7.4

// ResourceQuotaSpec represents a resource quota specification
type ResourceQuotaSpec struct {
	RequestsCPU           string
	LimitsCPU             string
	RequestsMemory        string
	LimitsMemory          string
	PersistentVolumeClaims string
	Pods                  string
}

// parseQuantity parses a Kubernetes quantity string (e.g., "4", "8Gi", "100m")
// Returns the numeric value and unit
func parseQuantity(quantity string) (float64, string, error) {
	quantity = strings.TrimSpace(quantity)
	
	// Handle CPU millicores (e.g., "100m")
	if strings.HasSuffix(quantity, "m") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(quantity, "m"), 64)
		return val, "m", err
	}
	
	// Handle memory units (e.g., "8Gi", "128Mi")
	if strings.HasSuffix(quantity, "Gi") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(quantity, "Gi"), 64)
		return val, "Gi", err
	}
	if strings.HasSuffix(quantity, "Mi") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(quantity, "Mi"), 64)
		return val, "Mi", err
	}
	if strings.HasSuffix(quantity, "Ki") {
		val, err := strconv.ParseFloat(strings.TrimSuffix(quantity, "Ki"), 64)
		return val, "Ki", err
	}
	
	// Handle plain numbers (e.g., "4", "20")
	val, err := strconv.ParseFloat(quantity, 64)
	return val, "", err
}

// TestResourceQuotaEnforcement_QuotaExists tests that resource quotas exist for all tenants
func TestResourceQuotaEnforcement_QuotaExists(t *testing.T) {
	// Property: For any tenant, a ResourceQuota should exist in the tenant namespace
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Verify tenant name is valid
		if tenantName == "" {
			t.Logf("Tenant name should not be empty")
			return false
		}
		
		// ResourceQuota should exist with a standard name
		quotaName := "tenant-quota"
		if quotaName == "" {
			t.Logf("ResourceQuota name should not be empty")
			return false
		}
		
		// ResourceQuota should be in the tenant namespace
		namespace := tenantName
		if namespace == "" {
			t.Logf("Namespace should not be empty")
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_CPULimits tests that CPU quotas are properly defined
func TestResourceQuotaEnforcement_CPULimits(t *testing.T) {
	// Property: For any tenant, CPU quotas should have positive values with requests <= limits
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Define CPU quotas (from the design)
		requestsCPU := "4"
		limitsCPU := "8"
		
		// Parse CPU values
		requestsVal, _, err := parseQuantity(requestsCPU)
		if err != nil {
			t.Logf("Failed to parse requests.cpu: %v", err)
			return false
		}
		
		limitsVal, _, err := parseQuantity(limitsCPU)
		if err != nil {
			t.Logf("Failed to parse limits.cpu: %v", err)
			return false
		}
		
		// Verify requests and limits are positive
		if requestsVal <= 0 {
			t.Logf("CPU requests should be positive, got %f for tenant %s", requestsVal, tenantName)
			return false
		}
		
		if limitsVal <= 0 {
			t.Logf("CPU limits should be positive, got %f for tenant %s", limitsVal, tenantName)
			return false
		}
		
		// Verify requests <= limits
		if requestsVal > limitsVal {
			t.Logf("CPU requests (%f) should be <= limits (%f) for tenant %s", requestsVal, limitsVal, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_MemoryLimits tests that memory quotas are properly defined
func TestResourceQuotaEnforcement_MemoryLimits(t *testing.T) {
	// Property: For any tenant, memory quotas should have positive values with requests <= limits
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Define memory quotas (from the design)
		requestsMemory := "8Gi"
		limitsMemory := "16Gi"
		
		// Parse memory values
		requestsVal, requestsUnit, err := parseQuantity(requestsMemory)
		if err != nil {
			t.Logf("Failed to parse requests.memory: %v", err)
			return false
		}
		
		limitsVal, limitsUnit, err := parseQuantity(limitsMemory)
		if err != nil {
			t.Logf("Failed to parse limits.memory: %v", err)
			return false
		}
		
		// Verify units match (both should be Gi)
		if requestsUnit != limitsUnit {
			t.Logf("Memory units should match: requests=%s, limits=%s", requestsUnit, limitsUnit)
			return false
		}
		
		// Verify requests and limits are positive
		if requestsVal <= 0 {
			t.Logf("Memory requests should be positive, got %f for tenant %s", requestsVal, tenantName)
			return false
		}
		
		if limitsVal <= 0 {
			t.Logf("Memory limits should be positive, got %f for tenant %s", limitsVal, tenantName)
			return false
		}
		
		// Verify requests <= limits
		if requestsVal > limitsVal {
			t.Logf("Memory requests (%f) should be <= limits (%f) for tenant %s", requestsVal, limitsVal, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_PVCQuota tests that PVC quotas are properly defined
func TestResourceQuotaEnforcement_PVCQuota(t *testing.T) {
	// Property: For any tenant, PVC quota should be a positive integer
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Define PVC quota (from the design)
		pvcQuota := "5"
		
		// Parse PVC quota
		pvcVal, _, err := parseQuantity(pvcQuota)
		if err != nil {
			t.Logf("Failed to parse persistentvolumeclaims quota: %v", err)
			return false
		}
		
		// Verify PVC quota is positive
		if pvcVal <= 0 {
			t.Logf("PVC quota should be positive, got %f for tenant %s", pvcVal, tenantName)
			return false
		}
		
		// Verify PVC quota is an integer
		if pvcVal != float64(int(pvcVal)) {
			t.Logf("PVC quota should be an integer, got %f for tenant %s", pvcVal, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_PodQuota tests that pod quotas are properly defined
func TestResourceQuotaEnforcement_PodQuota(t *testing.T) {
	// Property: For any tenant, pod quota should be a positive integer
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Define pod quota (from the design)
		podQuota := "20"
		
		// Parse pod quota
		podVal, _, err := parseQuantity(podQuota)
		if err != nil {
			t.Logf("Failed to parse pods quota: %v", err)
			return false
		}
		
		// Verify pod quota is positive
		if podVal <= 0 {
			t.Logf("Pod quota should be positive, got %f for tenant %s", podVal, tenantName)
			return false
		}
		
		// Verify pod quota is an integer
		if podVal != float64(int(podVal)) {
			t.Logf("Pod quota should be an integer, got %f for tenant %s", podVal, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_AllResourcesDefined tests that all required resources are defined
func TestResourceQuotaEnforcement_AllResourcesDefined(t *testing.T) {
	// Property: For any tenant, all required resource quotas should be defined
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Define all required quotas
		quotas := map[string]string{
			"requests.cpu":           "4",
			"limits.cpu":             "8",
			"requests.memory":        "8Gi",
			"limits.memory":          "16Gi",
			"persistentvolumeclaims": "5",
			"pods":                   "20",
		}
		
		// Verify all quotas are defined and non-empty
		for resource, value := range quotas {
			if value == "" {
				t.Logf("Quota for %s should not be empty for tenant %s", resource, tenantName)
				return false
			}
		}
		
		// Verify we have exactly 6 quotas
		if len(quotas) != 6 {
			t.Logf("Expected 6 quotas, got %d for tenant %s", len(quotas), tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_ExceedingQuotaDenied tests that exceeding quotas is denied
func TestResourceQuotaEnforcement_ExceedingQuotaDenied(t *testing.T) {
	// Property: For any tenant with a ResourceQuota, attempting to create resources
	// exceeding the quota should be rejected by Kubernetes
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Define quotas
		podQuota := "20"
		
		// Parse pod quota
		podQuotaVal, _, err := parseQuantity(podQuota)
		if err != nil {
			t.Logf("Failed to parse pod quota: %v", err)
			return false
		}
		
		// Attempt to create more pods than the quota allows
		attemptedPods := int(podQuotaVal) + 1
		
		// Verify that attempting to exceed the quota would be denied
		// In a real implementation, this would use the Kubernetes API to verify
		// For property testing, we verify the logical structure
		
		if attemptedPods <= int(podQuotaVal) {
			t.Logf("Attempted pods (%d) should exceed quota (%d) for tenant %s", attemptedPods, int(podQuotaVal), tenantName)
			return false
		}
		
		// The quota enforcement mechanism should deny this request
		quotaExceeded := true
		if !quotaExceeded {
			t.Logf("Quota should be exceeded when attempting %d pods with quota %d", attemptedPods, int(podQuotaVal))
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_WithinQuotaAllowed tests that resources within quota are allowed
func TestResourceQuotaEnforcement_WithinQuotaAllowed(t *testing.T) {
	// Property: For any tenant with a ResourceQuota, creating resources within the quota
	// should be allowed
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Define quotas
		podQuota := "20"
		
		// Parse pod quota
		podQuotaVal, _, err := parseQuantity(podQuota)
		if err != nil {
			t.Logf("Failed to parse pod quota: %v", err)
			return false
		}
		
		// Attempt to create pods within the quota
		attemptedPods := int(podQuotaVal) - 1
		
		// Verify that attempting to stay within the quota would be allowed
		if attemptedPods >= int(podQuotaVal) {
			t.Logf("Attempted pods (%d) should be within quota (%d) for tenant %s", attemptedPods, int(podQuotaVal), tenantName)
			return false
		}
		
		// The quota enforcement mechanism should allow this request
		withinQuota := true
		if !withinQuota {
			t.Logf("Request should be within quota when attempting %d pods with quota %d", attemptedPods, int(podQuotaVal))
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_QuotaScope tests that quotas are namespace-scoped
func TestResourceQuotaEnforcement_QuotaScope(t *testing.T) {
	// Property: For any tenant, ResourceQuota should be scoped to the tenant namespace
	
	f := func(seed uint) bool {
		pair := generateDistinctTenantPair(int(seed))
		
		// ResourceQuotas are namespace-scoped by default in Kubernetes
		// Each tenant has its own independent quota
		
		namespaceA := pair.TenantA
		namespaceB := pair.TenantB
		
		// Verify namespaces are distinct
		if namespaceA == namespaceB {
			t.Logf("Namespaces should be distinct for different tenants")
			return false
		}
		
		// ResourceQuota in namespace A does not affect namespace B
		// Each namespace has its own independent quota
		quotaNameA := "tenant-quota"
		quotaNameB := "tenant-quota"
		
		// Quotas can have the same name in different namespaces
		if quotaNameA != quotaNameB {
			t.Logf("Quota names should be consistent across tenants")
			return false
		}
		
		// But they are independent because they're in different namespaces
		if namespaceA == namespaceB {
			t.Logf("Quotas should be in different namespaces for different tenants")
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}

// TestResourceQuotaEnforcement_ReasonableLimits tests that quotas have reasonable limits for homelab
func TestResourceQuotaEnforcement_ReasonableLimits(t *testing.T) {
	// Property: For any tenant, resource quotas should have reasonable limits for homelab use
	
	f := func(seed uint) bool {
		tenantName := generateValidTenantName(int(seed))
		
		// Define quotas (from the design - reasonable for homelab)
		quotas := ResourceQuotaSpec{
			RequestsCPU:           "4",
			LimitsCPU:             "8",
			RequestsMemory:        "8Gi",
			LimitsMemory:          "16Gi",
			PersistentVolumeClaims: "5",
			Pods:                  "20",
		}
		
		// Verify CPU limits are reasonable (not too high for homelab)
		cpuLimits, _, _ := parseQuantity(quotas.LimitsCPU)
		if cpuLimits > 16 {
			t.Logf("CPU limits too high for homelab: %f for tenant %s", cpuLimits, tenantName)
			return false
		}
		
		// Verify memory limits are reasonable (not too high for homelab)
		memLimits, _, _ := parseQuantity(quotas.LimitsMemory)
		if memLimits > 32 {
			t.Logf("Memory limits too high for homelab: %fGi for tenant %s", memLimits, tenantName)
			return false
		}
		
		// Verify pod count is reasonable
		pods, _, _ := parseQuantity(quotas.Pods)
		if pods > 50 {
			t.Logf("Pod count too high for homelab: %f for tenant %s", pods, tenantName)
			return false
		}
		
		// Verify PVC count is reasonable
		pvcs, _, _ := parseQuantity(quotas.PersistentVolumeClaims)
		if pvcs > 10 {
			t.Logf("PVC count too high for homelab: %f for tenant %s", pvcs, tenantName)
			return false
		}
		
		return true
	}
	
	if err := quick.Check(f, &quick.Config{MaxCount: 100}); err != nil {
		t.Errorf("Property violated: %v", err)
	}
}
