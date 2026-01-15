# Model A Implementation Plan

## Current State Analysis

### What We Have (Aligned with Model A)
- ✅ Org-scoped EventListeners in `org-<org>` namespaces
- ✅ Pipeline execution in separate pipeline namespaces
- ✅ RepoBinding as product team interface
- ✅ Triggers created per RepoBinding in org namespace
- ✅ TriggerTemplates that create PipelineRuns
- ✅ Webhook secrets org-scoped
- ✅ Organization as primary tenant boundary
- ✅ Owner references for cascading deletion

### What Needs Work

#### 1. Controller Structure (991-line provisioners.go)
**Current:** Single `provisioners.go` with all provisioning logic mixed together
**Target:** Separate files scoped to CRD responsibilities

**Files:**
- `platform/onboarding/controller/controllers/provisioners.go` (991 lines)
- `platform/onboarding/controller/controllers/repobinding_controller.go`
- `platform/onboarding/controller/controllers/organization_controller.go`

#### 2. TriggerTemplates Are Too Thick
**Current:** `provisionTriggerTemplate()` in provisioners.go (lines ~665-770)
- Embeds pipeline resolution logic
- Hardcodes workspace setup
- Inline parameter mapping
- No versioning or catalog

**Target:** Thin dispatcher templates
- Reference from catalog
- Version-tagged
- Minimal parameter transformation
- No workflow logic

#### 3. No Template Catalog
**Current:** Templates created inline per RepoBinding
**Target:** Platform-managed template catalog in AphexPipelineResources

#### 4. Missing Canonical Parameter Contract
**Current:** Ad-hoc parameters (git-url, git-revision)
**Target:** Defined contract with required + optional params

#### 5. RepoBinding Schema Gaps
**Current Fields:**
```go
type RepoBindingSpec struct {
    AphexOrg          string
    RepoOrg           string
    RepoName          string
    PipelineName      string
    PermissionProfile string
}
```

**Missing:**
- `templateRef` (which dispatcher to use)
- `executionProfile` (standard/elevated as named profile)
- Template version selection

#### 6. No Execution Profile Abstraction
**Current:** `PermissionProfile` exists but not surfaced in templates
**Target:** Named execution profiles that templates apply

---

## Implementation Phases

### Phase 0: Controller Refactoring (Prerequisite)
**Goal:** Break 991-line provisioners.go into CRD-scoped files

**New Structure:**
```
controllers/
├── repobinding_controller.go          # Reconcile loop
├── repobinding_provisioners.go        # RepoBinding-specific provisioning
├── organization_controller.go         # Reconcile loop  
├── organization_provisioners.go       # Organization-specific provisioning
├── shared_provisioners.go             # Shared utilities
└── validators.go                      # Keep as-is
```

**Tasks:**
1. Create `repobinding_provisioners.go`
   - Move: provisionNamespace, provisionServiceAccount, provisionRBAC, provisionResourceQuota, provisionLimitRange, provisionNetworkPolicy, provisionTerraformBackendSecret
   - Move: provisionTrigger, provisionTriggerTemplate
   - Move: updateWebhookAllowlist, provisionWebhookSecret
   - Move: findPipelineNamespace helper

2. Create `organization_provisioners.go`
   - Move: provisionOrgNamespace, provisionEventListener, provisionTriggerBinding
   - Move: provisionCloudflareResources
   - Move: generateWebhookSecret helper

3. Create `shared_provisioners.go`
   - Move: buildRole (used by both)
   - Move: any other shared utilities

4. Update imports and method receivers

**Files to modify:**
- `platform/onboarding/controller/controllers/provisioners.go` → split
- `platform/onboarding/controller/controllers/repobinding_controller.go` → update imports
- `platform/onboarding/controller/controllers/organization_controller.go` → update imports

---

### Phase 1: Define Canonical Parameter Contract
**Goal:** Establish stable parameter interface between platform and pipelines

**Tasks:**
1. Create contract definition in `api/v1alpha1/pipeline_contract.go`
```go
// PipelineContract defines the canonical parameters
// that platform-triggered pipelines must accept
type PipelineContract struct {
    // Required parameters
    GitURL       string // $(params.git-url)
    GitRevision  string // $(params.git-revision)
    RepoFullName string // $(params.repo-full-name)
    EventType    string // $(params.event-type)
    EventID      string // $(params.event-id)
    
    // Optional platform-injected
    ExecutionProfile string // $(params.execution-profile)
    TriggeredAt      string // $(params.triggered-at)
    OrgName          string // $(params.org-name)
}
```

2. Document in `.kiro/docs/api.md` under new "Pipeline Contract" section

**Files to create:**
- `platform/onboarding/controller/api/v1alpha1/pipeline_contract.go`

**Files to update:**
- `.kiro/docs/api.md` (add Pipeline Contract section)

---

### Phase 2: Create Template Catalog in AphexPipelineResources
**Goal:** Platform-managed, versioned template catalog

**Tasks:**
1. Create template catalog structure
```
AphexPipelineResources/
└── templates/
    └── dispatchers/
        ├── run-pipeline-v1.yaml
        └── README.md
```

2. Create `run-pipeline-v1.yaml` - thin dispatcher template
```yaml
apiVersion: triggers.tekton.dev/v1beta1
kind: TriggerTemplate
metadata:
  name: run-pipeline-v1
spec:
  params:
    - name: git-url
    - name: git-revision
    - name: repo-full-name
    - name: event-type
    - name: event-id
    - name: pipeline-name
    - name: pipeline-namespace
    - name: execution-profile
      default: standard
  resourcetemplates:
    - apiVersion: tekton.dev/v1beta1
      kind: PipelineRun
      metadata:
        generateName: $(tt.params.pipeline-name)-
        namespace: $(tt.params.pipeline-namespace)
        labels:
          platform.arbiter.io/triggered: "true"
          platform.arbiter.io/event-type: $(tt.params.event-type)
      spec:
        pipelineRef:
          resolver: cluster
          params:
            - name: name
              value: $(tt.params.pipeline-name)
            - name: namespace
              value: $(tt.params.pipeline-namespace)
        params:
          - name: git-url
            value: $(tt.params.git-url)
          - name: git-revision
            value: $(tt.params.git-revision)
          - name: repo-full-name
            value: $(tt.params.repo-full-name)
          - name: event-type
            value: $(tt.params.event-type)
          - name: event-id
            value: $(tt.params.event-id)
          - name: execution-profile
            value: $(tt.params.execution-profile)
        serviceAccountName: pipeline-runner
        timeout: 1h
```

3. Document template catalog in `.kiro/docs/architecture.md`

**Files to create:**
- `AphexPipelineResources/templates/dispatchers/run-pipeline-v1.yaml`
- `AphexPipelineResources/templates/dispatchers/README.md`

**Files to update:**
- `.kiro/docs/architecture.md` (add Template Catalog section)

---

### Phase 3: Add Template Vending to Controllers
**Goal:** Controllers materialize templates from catalog into org namespaces

**Tasks:**
1. Create `template_catalog.go` in controllers
```go
type TemplateCatalog struct {
    templates map[string]string // name -> yaml content
}

func (c *TemplateCatalog) Get(name string) (string, error)
func (c *TemplateCatalog) List() []string
```

2. Embed templates in controller binary (or fetch from Git)
   - Use `//go:embed` to embed template YAML files
   - Parse at startup

3. Add `materializeTemplate()` method to organization provisioners
   - Takes template name + version
   - Creates TriggerTemplate in org namespace
   - Idempotent (apply semantics)

4. Update `provisionTriggerTemplate()` in repobinding provisioners
   - Remove inline template creation
   - Call `materializeTemplate()` instead
   - Reference materialized template by name

**Files to create:**
- `platform/onboarding/controller/controllers/template_catalog.go`

**Files to update:**
- `platform/onboarding/controller/controllers/repobinding_provisioners.go`
- `platform/onboarding/controller/controllers/organization_provisioners.go`

---

### Phase 4: Update RepoBinding CRD
**Goal:** Add templateRef and executionProfile fields

**Tasks:**
1. Update `repobinding_types.go`
```go
type RepoBindingSpec struct {
    AphexOrg          string `json:"aphexOrg"`
    RepoOrg           string `json:"repoOrg"`
    RepoName          string `json:"repoName"`
    PipelineName      string `json:"pipelineName"`
    
    // NEW: Template selection
    TemplateRef       string `json:"templateRef"`        // e.g. "run-pipeline-v1"
    
    // NEW: Execution profile (replaces PermissionProfile concept)
    ExecutionProfile  string `json:"executionProfile"`   // "standard" or "elevated"
}
```

2. Update CRD YAML
3. Update validators to check templateRef exists in catalog
4. Update provisioners to use templateRef
5. Deprecate PermissionProfile (keep for backward compat, map to ExecutionProfile)

**Files to update:**
- `platform/onboarding/controller/api/v1alpha1/repobinding_types.go`
- `platform/crds/repobinding-crd.yaml`
- `platform/onboarding/controller/controllers/validators.go`
- `platform/onboarding/controller/controllers/repobinding_provisioners.go`

---

### Phase 5: Implement Execution Profiles in Templates
**Goal:** Templates apply execution profiles (SA, timeouts, node selectors)

**Tasks:**
1. Define execution profiles in `execution_profiles.go`
```go
type ExecutionProfile struct {
    Name               string
    ServiceAccount     string
    Timeout            string
    NodeSelector       map[string]string
    ResourceLimits     corev1.ResourceRequirements
}

var Profiles = map[string]ExecutionProfile{
    "standard": {...},
    "elevated": {...},
}
```

2. Update template to inject profile settings
   - ServiceAccount from profile
   - Timeout from profile
   - Node selector from profile

3. Update provisioners to pass execution profile to template

**Files to create:**
- `platform/onboarding/controller/controllers/execution_profiles.go`

**Files to update:**
- `AphexPipelineResources/templates/dispatchers/run-pipeline-v1.yaml`
- `platform/onboarding/controller/controllers/repobinding_provisioners.go`

---

### Phase 6: Update Trigger Provisioning
**Goal:** Triggers reference catalog templates, not inline templates

**Tasks:**
1. Update `provisionTrigger()` in repobinding provisioners
   - Change template reference to use `templateRef` from RepoBinding
   - Ensure template exists in org namespace (materialize if needed)

2. Update Trigger spec to pass canonical params + execution profile

**Files to update:**
- `platform/onboarding/controller/controllers/repobinding_provisioners.go`

---

### Phase 7: Documentation Updates
**Goal:** Document Model A architecture in stable docs

**Tasks:**
1. Update `.kiro/docs/architecture.md`
   - Add "Template Catalog" section
   - Add "Execution Profiles" section
   - Add "Canonical Parameter Contract" section
   - Update "Trigger Model" section

2. Update `.kiro/docs/api.md`
   - Document Pipeline Contract
   - Document RepoBinding templateRef field
   - Document execution profiles

3. Update `.kiro/docs/data-models.md`
   - Update RepoBinding schema
   - Add ExecutionProfile schema

4. Update `.kiro/docs/operations.md`
   - Add "Adding New Templates" section
   - Add "Execution Profile Configuration" section

**Files to update:**
- `.kiro/docs/architecture.md`
- `.kiro/docs/api.md`
- `.kiro/docs/data-models.md`
- `.kiro/docs/operations.md`

---

## Implementation Order

1. **Phase 0** (Controller Refactoring) - Do first, unblocks everything
2. **Phase 1** (Canonical Contract) - Defines interface
3. **Phase 2** (Template Catalog) - Creates platform resources
4. **Phase 3** (Template Vending) - Wires catalog to controllers
5. **Phase 4** (RepoBinding CRD) - Updates API
6. **Phase 5** (Execution Profiles) - Implements profile system
7. **Phase 6** (Trigger Updates) - Completes integration
8. **Phase 7** (Documentation) - Captures final state

---

## Success Criteria

### Phase 0 Complete
- [ ] provisioners.go split into 3 files
- [ ] All tests pass
- [ ] No functional changes

### Phase 1 Complete ✅
- [x] PipelineContract type defined in AphexPipelineResources
- [x] Documented in PIPELINE_CONTRACT.md
- [x] Multi-repo support with array-based repo-full-name

### Phase 2 Complete ✅
- [x] run-pipeline-v1.yaml exists in AphexPipelineResources
- [x] Template is thin (no workflow logic)
- [x] Template uses canonical params
- [x] Template catalog documented

### Phase 3 Complete
- [ ] TemplateCatalog implemented
- [ ] Templates embedded in controller
- [ ] materializeTemplate() works

### Phase 4 Complete
- [ ] RepoBinding has templateRef field
- [ ] RepoBinding has executionProfile field
- [ ] CRD updated and applied
- [ ] Validators check templateRef

### Phase 5 Complete
- [ ] ExecutionProfile type defined
- [ ] Profiles applied in templates
- [ ] Standard and elevated profiles work

### Phase 6 Complete
- [ ] Triggers reference catalog templates
- [ ] Triggers pass execution profile
- [ ] End-to-end webhook → PipelineRun works

### Phase 7 Complete
- [ ] All 6 docs updated
- [ ] Model A architecture documented
- [ ] No stale content remains

---

## Migration Strategy

### Backward Compatibility
- Keep PermissionProfile field, map to ExecutionProfile internally
- Default templateRef to "run-pipeline-v1" if not specified
- Existing RepoBindings continue to work

### Rollout
1. Deploy Phase 0-3 (no API changes, backward compatible)
2. Deploy Phase 4 (CRD update, backward compatible with defaults)
3. Migrate existing RepoBindings to use templateRef explicitly
4. Deploy Phase 5-6 (execution profiles)
5. Update documentation (Phase 7)

---

## Next Steps

**Immediate:** Start Phase 0 - Controller Refactoring
- Create repobinding_provisioners.go
- Create organization_provisioners.go  
- Create shared_provisioners.go
- Move functions from provisioners.go
- Update imports
- Test

**After Phase 0:** Review and proceed to Phase 1
