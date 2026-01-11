package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// OrganizationSpec defines the desired state of Organization
type OrganizationSpec struct {
	// DisplayName is the human-readable organization name
	DisplayName string `json:"displayName"`

	// AdminUsers is a list of admin email addresses
	AdminUsers []string `json:"adminUsers"`

	// WebhookSecret is the GitHub webhook secret (auto-generated if not provided)
	// +optional
	WebhookSecret string `json:"webhookSecret,omitempty"`
}

// OrganizationStatus defines the observed state of Organization
type OrganizationStatus struct {
	// Namespace is the organization namespace name
	// +optional
	Namespace string `json:"namespace,omitempty"`

	// WebhookURL is the external webhook URL for GitHub integration
	// +optional
	WebhookURL string `json:"webhookURL,omitempty"`

	// Phase represents the organization provisioning status
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides human-readable status information
	// +optional
	Message string `json:"message,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Namespace",type=string,JSONPath=`.status.namespace`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Organization is the Schema for the organizations API
type Organization struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   OrganizationSpec   `json:"spec,omitempty"`
	Status OrganizationStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// OrganizationList contains a list of Organization
type OrganizationList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Organization `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Organization{}, &OrganizationList{})
}
