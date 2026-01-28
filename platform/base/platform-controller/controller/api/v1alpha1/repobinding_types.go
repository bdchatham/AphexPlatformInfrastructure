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
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9-]+$`
	RepoOrg string `json:"repoOrg"`

	// RepoName is the repository name
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-zA-Z0-9_.-]+$`
	RepoName string `json:"repoName"`

	// PipelineName is the name of the Pipeline to trigger when webhooks are received
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Pattern=`^[a-z0-9-]+$`
	PipelineName string `json:"pipelineName"`

	// TemplateRef is the name of the dispatcher template to use
	// +kubebuilder:validation:Required
	TemplateRef string `json:"templateRef"`

	// PipelineSpec is the raw YAML content of the Tekton Pipeline to create
	// The controller will create this Pipeline resource in the pipeline namespace
	// +kubebuilder:validation:Required
	PipelineSpec string `json:"pipelineSpec"`
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

	// PipelineCreated indicates if the Pipeline resource was created
	PipelineCreated bool `json:"pipelineCreated,omitempty"`

	// TriggerBindingCreated indicates if the EventListener namespace was updated
	TriggerBindingCreated bool `json:"triggerBindingCreated,omitempty"`

	// TriggerTemplateCreated indicates if the TriggerTemplate was created
	TriggerTemplateCreated bool `json:"triggerTemplateCreated,omitempty"`

	// TriggerCreated indicates if the Trigger was created
	TriggerCreated bool `json:"triggerCreated,omitempty"`

	// WebhookSecret is the generated webhook secret for GitHub webhook configuration
	WebhookSecret string `json:"webhookSecret,omitempty"`

	// WebhookURL is the webhook endpoint URL
	WebhookURL string `json:"webhookURL,omitempty"`

	// LastReconcileTime is the timestamp of the last reconciliation
	LastReconcileTime metav1.Time `json:"lastReconcileTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Org",type=string,JSONPath=`.spec.aphexOrg`
// +kubebuilder:printcolumn:name="Repo",type=string,JSONPath=`.spec.repoName`
// +kubebuilder:printcolumn:name="Pipeline",type=string,JSONPath=`.spec.pipelineName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

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
