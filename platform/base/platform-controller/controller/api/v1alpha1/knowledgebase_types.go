package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Repository defines a single repository to track
type Repository struct {
	// URL is the repository URL (supports GitHub, GitLab, Bitbucket, etc.)
	// +kubebuilder:validation:Required
	URL string `json:"url"`

	// Branch is the Git branch to track (default: main)
	// +optional
	Branch string `json:"branch,omitempty"`

	// Paths are the documentation paths to track (default: [".kiro/docs"])
	// Supports glob patterns (e.g., "docs/**/*.md", ".kiro/docs/**")
	// +optional
	Paths []string `json:"paths,omitempty"`
}

// MCPConfig defines the MCP server configuration
// If this field is set (non-nil), an MCP server will be provisioned
type MCPConfig struct {
	// Image is the container image for the MCP server
	// +kubebuilder:validation:Required
	Image string `json:"image"`

	// Port is the port the MCP server listens on
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port"`

	// QueryServiceURL is the URL of the query service
	// If empty, controller computes as http://query.{namespace}:8080
	// +optional
	QueryServiceURL string `json:"queryServiceURL,omitempty"`

	// Replicas is the number of MCP server replicas
	// +optional
	// +kubebuilder:validation:Minimum=1
	Replicas int32 `json:"replicas,omitempty"`
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

	// MCP configures the optional MCP server for this knowledge base
	// If set, an MCP server will be provisioned. If nil/omitted, no MCP server is created.
	// +optional
	MCP *MCPConfig `json:"mcp,omitempty"`
}

// MCPStatus defines the observed state of the MCP server
type MCPStatus struct {
	// Deployed indicates whether the MCP server is deployed
	// +optional
	Deployed bool `json:"deployed,omitempty"`

	// ServiceName is the Kubernetes service name for the MCP server
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// ServiceURL is the internal cluster URL for the MCP server
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// ReadyReplicas is the number of ready MCP server replicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
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

	// MCP contains the status of the MCP server (if enabled)
	// +optional
	MCP MCPStatus `json:"mcp,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="MCP",type=boolean,JSONPath=`.status.mcp.deployed`
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
