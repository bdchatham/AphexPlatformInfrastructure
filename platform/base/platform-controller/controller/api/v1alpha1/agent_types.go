package v1alpha1

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// ModelSpec defines the model server configuration
type ModelSpec struct {
	// Provider is the model serving backend (vllm, openai, anthropic, bedrock)
	// +kubebuilder:validation:Required
	// +kubebuilder:validation:Enum=vllm;openai;anthropic;bedrock
	Provider string `json:"provider"`

	// Name is the model identifier (e.g., meta-llama/Llama-3.1-70B-Instruct)
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Quantization method for the model (e.g., gptq_marlin, awq, fp16)
	// +optional
	Quantization string `json:"quantization,omitempty"`

	// GPUCount is the number of GPUs to allocate
	// +optional
	// +kubebuilder:validation:Minimum=1
	GPUCount int32 `json:"gpuCount,omitempty"`

	// Image is the container image for the model server
	// +optional
	Image string `json:"image,omitempty"`

	// Port is the model server port
	// +optional
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// KnowledgeBaseConfig references an existing KnowledgeBase
type KnowledgeBaseConfig struct {
	// Name is the KnowledgeBase resource name
	// +kubebuilder:validation:Required
	Name string `json:"name"`

	// Namespace is the KnowledgeBase namespace (defaults to Agent namespace)
	// +optional
	Namespace string `json:"namespace,omitempty"`
}

// OrchestrationConfig defines the orchestration layer
// If set, an orchestrator will be provisioned to combine model + KB
type OrchestrationConfig struct {
	// Image is the orchestrator container image
	// +optional
	Image string `json:"image,omitempty"`

	// Port is the orchestrator port
	// +optional
	// +kubebuilder:validation:Minimum=1024
	// +kubebuilder:validation:Maximum=65535
	Port int32 `json:"port,omitempty"`
}

// AgentSpec defines the desired state of Agent
type AgentSpec struct {
	// DisplayName is the human-readable agent name
	// +kubebuilder:validation:Required
	DisplayName string `json:"displayName"`

	// Description provides context about this agent
	// +optional
	Description string `json:"description,omitempty"`

	// Model defines the model server configuration (always provisioned)
	// +kubebuilder:validation:Required
	Model ModelSpec `json:"model"`

	// KnowledgeBase references an existing KnowledgeBase for RAG
	// +optional
	KnowledgeBase *KnowledgeBaseConfig `json:"knowledgeBase,omitempty"`

	// Orchestration configures the orchestration layer
	// If set, an orchestrator will be provisioned. If nil, no orchestrator.
	// +optional
	Orchestration *OrchestrationConfig `json:"orchestration,omitempty"`
}

// ModelServerStatus defines the observed state of the model server
type ModelServerStatus struct {
	// Deployed indicates whether the model server is deployed
	// +optional
	Deployed bool `json:"deployed,omitempty"`

	// ServiceName is the Kubernetes service name for the model server
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// ServiceURL is the internal cluster URL for the model server
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// ReadyReplicas is the number of ready model server replicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// OrchestratorStatus defines the observed state of the orchestrator
type OrchestratorStatus struct {
	// Deployed indicates whether the orchestrator is deployed
	// +optional
	Deployed bool `json:"deployed,omitempty"`

	// ServiceName is the Kubernetes service name for the orchestrator
	// +optional
	ServiceName string `json:"serviceName,omitempty"`

	// ServiceURL is the internal cluster URL for the orchestrator
	// +optional
	ServiceURL string `json:"serviceURL,omitempty"`

	// ReadyReplicas is the number of ready orchestrator replicas
	// +optional
	ReadyReplicas int32 `json:"readyReplicas,omitempty"`
}

// AgentStatus defines the observed state of Agent
type AgentStatus struct {
	// Phase represents the current state (Pending, Ready, Failed)
	// +optional
	Phase string `json:"phase,omitempty"`

	// Message provides human-readable status information
	// +optional
	Message string `json:"message,omitempty"`

	// LastReconcileTime is the timestamp of the last reconciliation
	// +optional
	LastReconcileTime *metav1.Time `json:"lastReconcileTime,omitempty"`

	// ModelServer contains the status of the model server
	// +optional
	ModelServer ModelServerStatus `json:"modelServer,omitempty"`

	// Orchestrator contains the status of the orchestrator (if enabled)
	// +optional
	Orchestrator *OrchestratorStatus `json:"orchestrator,omitempty"`
}

// +kubebuilder:object:root=true
// +kubebuilder:subresource:status
// +kubebuilder:resource:scope=Namespaced
// +kubebuilder:printcolumn:name="Display Name",type=string,JSONPath=`.spec.displayName`
// +kubebuilder:printcolumn:name="Model",type=string,JSONPath=`.spec.model.name`
// +kubebuilder:printcolumn:name="Phase",type=string,JSONPath=`.status.phase`
// +kubebuilder:printcolumn:name="Age",type=date,JSONPath=`.metadata.creationTimestamp`

// Agent is the Schema for the agents API
type Agent struct {
	metav1.TypeMeta   `json:",inline"`
	metav1.ObjectMeta `json:"metadata,omitempty"`

	Spec   AgentSpec   `json:"spec,omitempty"`
	Status AgentStatus `json:"status,omitempty"`
}

// +kubebuilder:object:root=true

// AgentList contains a list of Agent
type AgentList struct {
	metav1.TypeMeta `json:",inline"`
	metav1.ListMeta `json:"metadata,omitempty"`
	Items           []Agent `json:"items"`
}

func init() {
	SchemeBuilder.Register(&Agent{}, &AgentList{})
}
