// Package constants defines all shared constants used across the platform controller.
// This centralizes magic strings and configuration defaults to improve maintainability.
package constants

import "time"

// Finalizers used by controllers to ensure cleanup before resource deletion.
const (
	RepoBindingFinalizer   = "platform.aphex/finalizer"
	OrganizationFinalizer  = "platform.aphex/organization-finalizer"
	AgentFinalizer         = "platform.aphex/agent-finalizer"
	KnowledgeBaseFinalizer = "platform.aphex/knowledgebase-finalizer"
)

// Service account names used for pipeline execution and event handling.
const (
	PipelineRunnerServiceAccount      = "pipeline-runner"
	EventListenerServiceAccount       = "eventlistener"
	ESOSecretsReaderAccount           = "eso-secrets-reader"
	PlatformControllerServiceAccount  = "platform-controller"
)

// Namespace names and defaults.
const (
	DefaultPlatformNamespace  = "platform-system"
	DefaultAllowlistNamespace = "pipeline-system"
	ArgoCDNamespace           = "argocd"
)

// Environment variable names for configuration.
const (
	EnvPlatformNamespace    = "PLATFORM_NAMESPACE"
	EnvAllowlistNamespace   = "ALLOWLIST_NAMESPACE"
	EnvDevelopmentMode      = "DEVELOPMENT_MODE"
	EnvLeaderElectionEnable = "LEADER_ELECTION_ENABLED"
)

// Label keys used for resource tracking and organization.
const (
	LabelManagedBy     = "platform.aphex/managed-by"
	LabelPipeline      = "platform.aphex/pipeline"
	LabelOrganization  = "platform.aphex/organization"
	LabelRepo          = "platform.aphex/repo"
	LabelOwnerName     = "platform.aphex/owner-name"
	LabelOwnerKind     = "platform.aphex/owner-kind"
	LabelAphexOrg      = "aphex.dev/org"
)

// Label values for managed-by labels.
const (
	ManagedByPlatformController     = "platform-controller"
	ManagedByOrganizationController = "organization-controller"
	ManagedByAgentController        = "agent-controller"
	ManagedByKnowledgeBaseController = "knowledgebase-controller"
)

// Annotation keys.
const (
	AnnotationLastAppliedSpec = "platform.aphex/last-applied-spec"
)

// Resource status phases.
const (
	PhasePending      = "Pending"
	PhaseProvisioning = "Provisioning"
	PhaseReady        = "Ready"
	PhaseActive       = "Active"
	PhaseFailed       = "Failed"
	PhaseDeleting     = "Deleting"
)

// Timeouts and delays for controller operations.
const (
	DefaultProvisioningTimeout = 5 * time.Minute
	DefaultDependencyWaitTime  = 30 * time.Second
	DefaultShutdownTimeout     = 30 * time.Second
	DefaultHealthThreshold     = 5 * time.Minute
)

// Rate limiting configuration.
const (
	DefaultMaxConcurrentReconciles = 3
	DefaultBaseDelay               = 1 * time.Second
	DefaultMaxDelay                = 60 * time.Second
	DefaultStatusRetryCount        = 3
)

// Resource names used in provisioning.
const (
	TenantQuotaName           = "tenant-quota"
	TenantLimitsName          = "tenant-limits"
	TenantIsolationPolicyName = "tenant-isolation"
	TerraformBackendSecretName = "terraform-backend-config"
	WebhookSecretName         = "github-webhook-secret"
	AllowlistConfigMapName    = "repo-allowlist"
	CloudflareAPITokenSecret  = "cloudflare-api-token"
)

// RBAC resource names.
const (
	PipelineRunnerRoleName        = "pipeline-runner"
	OrganizationAdminRoleName     = "organization-admin"
	ArgoCDApplicationDeployerRole = "argocd-application-deployer"
	EventListenerAccessRole       = "eventlistener-access"
)

// Tekton resource names.
const (
	GitHubListenerName     = "github-listener"
	GitHubPushBindingName  = "github-push-binding"
)

// ArgoCD GVK constants.
const (
	ArgoCDGroup      = "argoproj.io"
	ArgoCDVersion    = "v1alpha1"
	ArgoCDAppKind    = "Application"
	ArgoCDProjectKind = "AppProject"
)

// Tekton GVK constants.
const (
	TektonPipelineGroup   = "tekton.dev"
	TektonPipelineVersion = "v1"
	TektonTriggersGroup   = "triggers.tekton.dev"
	TektonTriggersVersion = "v1beta1"
)

// Default resource quota values.
const (
	DefaultQuotaCPURequests    = "16"
	DefaultQuotaCPULimits      = "32"
	DefaultQuotaMemoryRequests = "32Gi"
	DefaultQuotaMemoryLimits   = "48Gi"
	DefaultQuotaPVCs           = "5"
	DefaultQuotaPods           = "20"
)

// Default limit range values.
const (
	DefaultLimitCPU           = "500m"
	DefaultLimitMemory        = "512Mi"
	DefaultRequestCPU         = "100m"
	DefaultRequestMemory      = "128Mi"
	DefaultMaxCPU             = "16"
	DefaultMaxMemory          = "32Gi"
	DefaultMinCPU             = "50m"
	DefaultMinMemory          = "64Mi"
)

// Controller names for logging and metrics.
const (
	ControllerNameRepoBinding   = "RepoBinding"
	ControllerNameOrganization  = "Organization"
	ControllerNameAgent         = "Agent"
	ControllerNameKnowledgeBase = "KnowledgeBase"
)

// Leader election configuration.
const (
	LeaderElectionID = "platform-controller.platform.aphex"
)

// Metrics configuration.
const (
	MetricsBindAddress = ":8080"
	ProbeBindAddress   = ":8081"
)

// Webhook domain configuration.
const (
	WebhookDomain = "arbiter-dev.com"
)
