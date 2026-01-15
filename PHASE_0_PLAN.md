# Phase 0: Controller Refactoring - Execution Plan

## Current File Analysis

### provisioners.go (991 lines)
**RepoBindingReconciler methods (19):**
- provisionNamespace
- provisionServiceAccount
- provisionRBAC
- provisionRole
- provisionRoleBinding
- provisionClusterRole
- provisionClusterRoleBinding
- buildRole
- provisionResourceLimits
- provisionResourceQuota
- provisionLimitRange
- provisionNetworkPolicy
- provisionTerraformBackendSecret
- updateEventListenerNamespaces
- provisionTriggerTemplate
- provisionTrigger
- provisionAllowlistEntry
- updateRepoBindingStatusWithWebhookInfo
- findPipelineNamespace

**Shared functions:**
- setOwnerReference (line 63)
- stringPtr helper (likely exists)

## Target Structure

```
controllers/
├── repobinding_controller.go          # Existing - reconcile loop
├── repobinding_provisioners.go        # NEW - RepoBinding provisioning
├── organization_controller.go         # Existing - reconcile loop
├── organization_provisioners.go       # NEW - Organization provisioning  
├── shared.go                          # NEW - shared utilities
├── validators.go                      # Existing - keep as-is
└── validators_test.go                 # Existing - keep as-is
```

## File Contents

### repobinding_provisioners.go
**Purpose:** All RepoBinding-specific provisioning logic

**Contents:**
- provisionNamespace
- provisionServiceAccount  
- provisionRBAC
  - provisionRole
  - provisionRoleBinding
  - provisionClusterRole
  - provisionClusterRoleBinding
- provisionResourceLimits
  - provisionResourceQuota
  - provisionLimitRange
- provisionNetworkPolicy
- provisionTerraformBackendSecret
- provisionTrigger
- provisionTriggerTemplate
- updateEventListenerNamespaces
- provisionAllowlistEntry
- updateRepoBindingStatusWithWebhookInfo
- findPipelineNamespace (helper)

**Estimated lines:** ~700

### organization_provisioners.go
**Purpose:** All Organization-specific provisioning logic

**Contents:**
- Extract from organization_controller.go:
  - provisionOrgNamespace
  - provisionEventListener
  - provisionTriggerBinding
  - provisionCloudflareResources
  - generateWebhookSecret
  - Any other org-specific helpers

**Estimated lines:** ~200

### shared.go
**Purpose:** Utilities used by both controllers

**Contents:**
- setOwnerReference
- stringPtr
- buildRole (used by both RepoBinding and potentially Organization)
- Any other shared helpers

**Estimated lines:** ~100

## Implementation Steps

### Step 1: Create repobinding_provisioners.go
1. Copy package declaration and imports from provisioners.go
2. Copy all RepoBindingReconciler methods (lines 26-991)
3. Ensure all imports are present
4. Add file header comment

### Step 2: Create shared.go
1. Copy shared functions:
   - setOwnerReference
   - stringPtr
   - buildRole
2. Update to work with both reconciler types

### Step 3: Extract organization provisioners
1. Review organization_controller.go
2. Identify provisioning methods
3. Create organization_provisioners.go
4. Move methods

### Step 4: Clean up provisioners.go
1. Delete provisioners.go (all content moved)

### Step 5: Update imports
1. repobinding_controller.go - verify imports
2. organization_controller.go - verify imports
3. Run `go mod tidy`

### Step 6: Test
1. Run `go build`
2. Run tests
3. Verify no functional changes

## Execution

Ready to execute? I'll:
1. Create repobinding_provisioners.go with all RepoBinding methods
2. Create shared.go with shared utilities
3. Review organization_controller.go and create organization_provisioners.go
4. Delete old provisioners.go
5. Test compilation

Proceed?
