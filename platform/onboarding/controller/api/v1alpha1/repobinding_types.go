package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// RepoBindingSpec defines the desired state of RepoBinding
type RepoBindingSpec struct {
	// AphexOrg is the name of the Organization resource to bind to
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	AphexOrg string `json:"aphexOrg"`

	// RepoOrg is the GitHub organization name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	RepoOrg string `json:"repoOrg"`

	// RepoName is the repository name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	RepoName string `json:"repoName"`

	// TenantName is the tenant namespace name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	TenantName string `json:"tenantName"`

	// PipelineName is the name of the Pipeline to trigger when webhooks are received
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	PipelineName string `json:"pipelineName"`

	// PermissionProfile defines the RBAC permission level
	// +kubebuilder:validation:Enum=standard;elevated
	// +kubebuilder:default=standard
	PermissionProfile string `json:"permissionProfile,omitempty"`
}

// RepoBindingStatus defines the observed state of RepoBinding
type RepoBindingStatus struct {
	// Phase represents the current state of the onboarding process
	// +kubebuilder:validation:Enum=Pending;Provisioning;Ready;Failed
	Phase string `json:"phase,omitempty"`

	// Message provides additional context about the current phase
	Message string `json:"message,omitempty"`

	// NamespaceCreated indicates if the tenant namespace was created
	NamespaceCreated bool `json:"namespaceCreated,omitempty"`

	// ServiceAccountCreated indicates if the service account was created
	ServiceAccountCreated bool `json:"serviceAccountCreated,omitempty"`

	// RBACCreated indicates if RBAC was configured
	RBACCreated bool `json:"rbacCreated,omitempty"`

	// QuotasCreated indicates if ResourceQuota and LimitRange were created
	QuotasCreated bool `json:"quotasCreated,omitempty"`

	// NetworkPolicyCreated indicates if the NetworkPolicy was created
	NetworkPolicyCreated bool `json:"networkPolicyCreated,omitempty"`

	// TerraformSecretCreated indicates if the Terraform backend secret was created
	TerraformSecretCreated bool `json:"terraformSecretCreated,omitempty"`

	// AllowlistUpdated indicates if the repository was added to the allowlist
	AllowlistUpdated bool `json:"allowlistUpdated,omitempty"`

	// EventListenerCreated indicates if the EventListener was created
	EventListenerCreated bool `json:"eventListenerCreated,omitempty"`

	// TriggerBindingCreated indicates if the TriggerBinding was created
	TriggerBindingCreated bool `json:"triggerBindingCreated,omitempty"`

	// TriggerTemplateCreated indicates if the TriggerTemplate was created
	TriggerTemplateCreated bool `json:"triggerTemplateCreated,omitempty"`

	// TriggerCreated indicates if the Trigger was created
	TriggerCreated bool `json:"triggerCreated,omitempty"`

	// WebhookSecret is the generated webhook secret for GitHub webhook configuration
	WebhookSecret string `json:"webhookSecret,omitempty"`

	// WebhookURL is the Lighthouse webhook endpoint URL
	WebhookURL string `json:"webhookURL,omitempty"`

	// LastReconcileTime is the timestamp of the last reconciliation
	LastReconcileTime metav1.Time `json:"lastReconcileTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced

// RepoBinding is the Schema for the repobindings API
type RepoBinding struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   RepoBindingSpec   `json:"spec,omitempty"`
	Status RepoBindingStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// RepoBindingList contains a list of RepoBinding
type RepoBindingList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []RepoBinding `json:"items"`
}

func init() {
	SchemeBuilder.Register(&RepoBinding{}, &RepoBindingList{})
}
