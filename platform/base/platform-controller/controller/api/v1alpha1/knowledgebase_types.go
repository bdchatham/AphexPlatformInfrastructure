package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Repository defines a single repository to track
type Repository struct {
	// URL is the GitHub repository URL (https://github.com/org/repo)
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Branch is the Git branch to track (default: main)
	// +optional
	Branch string `json:"branch,omitempty"`

	// Paths are the documentation paths to track (default: [".kiro/docs"])
	// +optional
	Paths []string `json:"paths,omitempty"`
}

// KnowledgeBaseSpec defines the desired state of KnowledgeBase
type KnowledgeBaseSpec struct {
	// DisplayName is the human-readable knowledge base name
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Description provides context about this knowledge base
	// +optional
	Description string `json:"description,omitempty"`

	// Repositories is the list of repositories to track
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:MinItems=1
	Repositories []Repository `json:"repositories"`
}

// KnowledgeBaseStatus defines the observed state of KnowledgeBase
type KnowledgeBaseStatus struct {
	// Phase represents the current state (Pending, Ready, Failed)
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides human-readable status information
	// +optional
	Message string `json:"message,omitempty"`

	// LastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// KnowledgeBase is the Schema for the knowledgebases API
type KnowledgeBase struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   KnowledgeBaseSpec   `json:"spec,omitempty"`
	Status KnowledgeBaseStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// KnowledgeBaseList contains a list of KnowledgeBase
type KnowledgeBaseList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []KnowledgeBase `json:"items"`
}

func init() {
	SchemeBuilder.Register(&KnowledgeBase{}, &KnowledgeBaseList{})
}
