package controllers

import (
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	triggersv1beta1 "github.com/tektoncd/triggers/pkg/apis/triggers/v1beta1"
)

// DispatcherTemplate represents a thin dispatcher template that converts
// webhook events into PipelineRuns
type DispatcherTemplate struct {
	Name    string
	Version string
	Spec    triggersv1beta1.TriggerTemplateSpec
}

// NewRunPipelineV1 creates the run-pipeline-v1 dispatcher template
func NewRunPipelineV1() *DispatcherTemplate {
	return &DispatcherTemplate{
		Name:    "run-pipeline-v1",
		Version: "v1",
		Spec: triggersv1beta1.TriggerTemplateSpec{
			Params: []triggersv1beta1.ParamSpec{
				{Name: "git-url", Description: "Repository clone URL"},
				{Name: "git-revision", Description: "Git commit SHA or branch"},
				{Name: "repo-full-name", Description: "Repository name (org/repo)"},
				{Name: "event-type", Description: "Webhook event type"},
				{Name: "event-id", Description: "Unique event identifier"},
				{Name: "pipeline-name", Description: "Name of the pipeline to run"},
				{Name: "pipeline-namespace", Description: "Namespace where pipeline exists"},
				{Name: "execution-profile", Description: "Execution profile (standard/elevated)", Default: stringPtr("standard")},
				{Name: "org-name", Description: "Organization name", Default: stringPtr("")},
				{Name: "triggered-at", Description: "Webhook timestamp", Default: stringPtr("")},
			},
			ResourceTemplates: []triggersv1beta1.TriggerResourceTemplate{
				{
					RawExtension: runtime.RawExtension{
						Raw: buildPipelineRunTemplate(),
					},
				},
			},
		},
	}
}

// ToTriggerTemplate converts a DispatcherTemplate to a Tekton TriggerTemplate
func (t *DispatcherTemplate) ToTriggerTemplate(namespace string, orgName string) *triggersv1beta1.TriggerTemplate {
	return &triggersv1beta1.TriggerTemplate{
		TypeMeta: metav1.TypeMeta{
			APIVersion: "triggers.tekton.dev/v1beta1",
			Kind:       "TriggerTemplate",
		},
		ObjectMeta: metav1.ObjectMeta{
			Name:      t.Name,
			Namespace: namespace,
			Labels: map[string]string{
				"platform.arbiter.io/template-version": t.Version,
				"platform.arbiter.io/managed-by":       "platform",
				"platform.arbiter.io/organization":     orgName,
			},
		},
		Spec: t.Spec,
	}
}

// buildPipelineRunTemplate creates the PipelineRun resource template
func buildPipelineRunTemplate() []byte {
	return []byte(`{
			"apiVersion": "tekton.dev/v1",
			"kind": "PipelineRun",
			"metadata": {
				"generateName": "$(tt.params.pipeline-name)-",
				"namespace": "$(tt.params.pipeline-namespace)",
				"labels": {
					"platform.arbiter.io/triggered": "true",
					"platform.arbiter.io/event-type": "$(tt.params.event-type)",
					"platform.arbiter.io/event-id": "$(tt.params.event-id)",
					"platform.arbiter.io/execution-profile": "$(tt.params.execution-profile)"
				}
			},
			"spec": {
				"pipelineRef": {
					"resolver": "cluster",
					"params": [
						{"name": "name", "value": "$(tt.params.pipeline-name)"},
						{"name": "namespace", "value": "$(tt.params.pipeline-namespace)"}
					]
				},
				"params": [
					{"name": "git-url", "value": "$(tt.params.git-url)"},
					{"name": "git-revision", "value": "$(tt.params.git-revision)"},
					{"name": "repo-full-name", "value": ["$(tt.params.repo-full-name)"]},
					{"name": "event-type", "value": "$(tt.params.event-type)"},
					{"name": "event-id", "value": "$(tt.params.event-id)"},
					{"name": "execution-profile", "value": "$(tt.params.execution-profile)"},
					{"name": "triggered-at", "value": "$(tt.params.triggered-at)"},
					{"name": "org-name", "value": "$(tt.params.org-name)"}
				],
				"serviceAccountName": "pipeline-runner",
				"timeout": "1h"
			}
		}`)
}
