package controllers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/bdchatham/AphexPlatformInfrastructure/platform/platform-controller/controller/api/v1alpha1"
)

func TestValidateRepoBinding(t *testing.T) {
	tests := []struct {
		name    string
		rb      *platformv1alpha1.RepoBinding
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid repobinding with standard profile",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "bdchatham",
					RepoName:     "test-repo",
					PipelineName: "tenant-test",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: false,
		},
		{
			name: "valid repobinding with elevated profile",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "bdchatham",
					RepoName:     "test-repo",
					PipelineName: "tenant-test",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: false,
		},
		{
			name: "valid repobinding with empty profile defaults to standard",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "bdchatham",
					RepoName:     "test-repo",
					PipelineName: "tenant-test",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: false,
		},
		{
			name: "invalid org not in approved list",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "unapproved-org",
					RepoName:     "test-repo",
					PipelineName: "tenant-test",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: true,
			errMsg:  "not in approved list",
		},
		{
			name: "invalid tenant name with uppercase",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "bdchatham",
					RepoName:     "test-repo",
					PipelineName: "Tenant-Test",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: true,
			errMsg:  "must match pattern",
		},
		{
			name: "invalid tenant name with special characters",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "bdchatham",
					RepoName:     "test-repo",
					PipelineName: "tenant_test",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: true,
			errMsg:  "must match pattern",
		},
		{
			name: "invalid privileged namespace kube-system",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "bdchatham",
					RepoName:     "test-repo",
					PipelineName: "kube-system",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: true,
			errMsg:  "privileged name",
		},
		{
			name: "invalid privileged namespace default",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "bdchatham",
					RepoName:     "test-repo",
					PipelineName: "default",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: true,
			errMsg:  "privileged name",
		},
		{
			name: "invalid permission profile",
			rb: &platformv1alpha1.RepoBinding{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-binding",
					Namespace: "platform-system",
				},
				Spec: platformv1alpha1.RepoBindingSpec{
					AphexOrg:     "test-org",
					RepoOrg:      "bdchatham",
					RepoName:     "test-repo",
					PipelineName: "tenant-test",
					TemplateRef:  "run-pipeline-v1",
					PipelineSpec: "apiVersion: tekton.dev/v1\nkind: Pipeline\nmetadata:\n  name: test\nspec: {}",
				},
			},
			wantErr: true,
			errMsg:  "must be 'standard' or 'elevated'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			// Create catalog with default templates
			catalog := NewTemplateCatalog()
			err := ValidateRepoBinding(tt.rb, catalog)
			if (err != nil) != tt.wantErr {
				t.Errorf("ValidateRepoBinding() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("ValidateRepoBinding() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(s) > len(substr) && (s[:len(substr)] == substr || s[len(s)-len(substr):] == substr || containsMiddle(s, substr)))
}

func containsMiddle(s, substr string) bool {
	for i := 0; i <= len(s)-len(substr); i++ {
		if s[i:i+len(substr)] == substr {
			return true
		}
	}
	return false
}

func TestIsValidBranchName(t *testing.T) {
	tests := []struct {
		name   string
		branch string
		want   bool
	}{
		{
			name:   "valid branch name main",
			branch: "main",
			want:   true,
		},
		{
			name:   "valid branch name with slashes",
			branch: "feature/new-feature",
			want:   true,
		},
		{
			name:   "valid branch name with dashes",
			branch: "feature-branch",
			want:   true,
		},
		{
			name:   "valid branch name with underscores",
			branch: "feature_branch",
			want:   true,
		},
		{
			name:   "invalid branch name with space",
			branch: "feature branch",
			want:   false,
		},
		{
			name:   "invalid branch name with tilde",
			branch: "feature~branch",
			want:   false,
		},
		{
			name:   "invalid branch name with caret",
			branch: "feature^branch",
			want:   false,
		},
		{
			name:   "invalid branch name with colon",
			branch: "feature:branch",
			want:   false,
		},
		{
			name:   "invalid branch name with question mark",
			branch: "feature?branch",
			want:   false,
		},
		{
			name:   "invalid branch name with asterisk",
			branch: "feature*branch",
			want:   false,
		},
		{
			name:   "invalid branch name with bracket",
			branch: "feature[branch",
			want:   false,
		},
		{
			name:   "invalid branch name with backslash",
			branch: "feature\\branch",
			want:   false,
		},
		{
			name:   "invalid branch name with double dot",
			branch: "feature..branch",
			want:   false,
		},
		{
			name:   "invalid branch name with @{",
			branch: "feature@{branch",
			want:   false,
		},
		{
			name:   "invalid branch name with double slash",
			branch: "feature//branch",
			want:   false,
		},
		{
			name:   "invalid branch name starting with slash",
			branch: "/feature",
			want:   false,
		},
		{
			name:   "invalid branch name ending with slash",
			branch: "feature/",
			want:   false,
		},
		{
			name:   "invalid branch name starting with dot",
			branch: ".feature",
			want:   false,
		},
		{
			name:   "invalid branch name ending with dot",
			branch: "feature.",
			want:   false,
		},
		{
			name:   "invalid branch name ending with .lock",
			branch: "feature.lock",
			want:   false,
		},
		{
			name:   "invalid empty branch name",
			branch: "",
			want:   false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := isValidBranchName(tt.branch); got != tt.want {
				t.Errorf("isValidBranchName() = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestValidateKnowledgeBaseSpec(t *testing.T) {
	tests := []struct {
		name    string
		kb      *platformv1alpha1.KnowledgeBase
		wantErr bool
		errMsg  string
	}{
		{
			name: "valid knowledge base with single repository",
			kb: &platformv1alpha1.KnowledgeBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kb",
					Namespace: "default",
				},
				Spec: platformv1alpha1.KnowledgeBaseSpec{
					DisplayName: "Test KB",
					Repositories: []platformv1alpha1.Repository{
						{
							URL:    "https://github.com/bdchatham/test-repo",
							Branch: "main",
							Paths:  []string{".kiro/docs"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid knowledge base with multiple repositories",
			kb: &platformv1alpha1.KnowledgeBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kb",
					Namespace: "default",
				},
				Spec: platformv1alpha1.KnowledgeBaseSpec{
					DisplayName: "Test KB",
					Repositories: []platformv1alpha1.Repository{
						{
							URL:    "https://github.com/bdchatham/repo1",
							Branch: "main",
							Paths:  []string{".kiro/docs"},
						},
						{
							URL:    "https://github.com/bdchatham/repo2",
							Branch: "develop",
							Paths:  []string{".kiro/docs/api"},
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "valid knowledge base with default branch and paths",
			kb: &platformv1alpha1.KnowledgeBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kb",
					Namespace: "default",
				},
				Spec: platformv1alpha1.KnowledgeBaseSpec{
					DisplayName: "Test KB",
					Repositories: []platformv1alpha1.Repository{
						{
							URL: "https://github.com/bdchatham/test-repo",
						},
					},
				},
			},
			wantErr: false,
		},
		{
			name: "invalid knowledge base with empty repositories",
			kb: &platformv1alpha1.KnowledgeBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kb",
					Namespace: "default",
				},
				Spec: platformv1alpha1.KnowledgeBaseSpec{
					DisplayName:  "Test KB",
					Repositories: []platformv1alpha1.Repository{},
				},
			},
			wantErr: true,
			errMsg:  "repositories array cannot be empty",
		},
		{
			name: "invalid repository URL not starting with https://github.com/",
			kb: &platformv1alpha1.KnowledgeBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kb",
					Namespace: "default",
				},
				Spec: platformv1alpha1.KnowledgeBaseSpec{
					DisplayName: "Test KB",
					Repositories: []platformv1alpha1.Repository{
						{
							URL:    "https://gitlab.com/bdchatham/test-repo",
							Branch: "main",
							Paths:  []string{".kiro/docs"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "URL must start with https://github.com/",
		},
		{
			name: "invalid repository URL with http instead of https",
			kb: &platformv1alpha1.KnowledgeBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kb",
					Namespace: "default",
				},
				Spec: platformv1alpha1.KnowledgeBaseSpec{
					DisplayName: "Test KB",
					Repositories: []platformv1alpha1.Repository{
						{
							URL:    "http://github.com/bdchatham/test-repo",
							Branch: "main",
							Paths:  []string{".kiro/docs"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "URL must start with https://github.com/",
		},
		{
			name: "invalid branch name with spaces",
			kb: &platformv1alpha1.KnowledgeBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kb",
					Namespace: "default",
				},
				Spec: platformv1alpha1.KnowledgeBaseSpec{
					DisplayName: "Test KB",
					Repositories: []platformv1alpha1.Repository{
						{
							URL:    "https://github.com/bdchatham/test-repo",
							Branch: "feature branch",
							Paths:  []string{".kiro/docs"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "invalid branch name",
		},
		{
			name: "invalid path not starting with .kiro/docs",
			kb: &platformv1alpha1.KnowledgeBase{
				ObjectMeta: metav1.ObjectMeta{
					Name:      "test-kb",
					Namespace: "default",
				},
				Spec: platformv1alpha1.KnowledgeBaseSpec{
					DisplayName: "Test KB",
					Repositories: []platformv1alpha1.Repository{
						{
							URL:    "https://github.com/bdchatham/test-repo",
							Branch: "main",
							Paths:  []string{"docs"},
						},
					},
				},
			},
			wantErr: true,
			errMsg:  "path must start with .kiro/docs",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			reconciler := &KnowledgeBaseReconciler{}
			err := reconciler.validateSpec(tt.kb)
			if (err != nil) != tt.wantErr {
				t.Errorf("validateSpec() error = %v, wantErr %v", err, tt.wantErr)
				return
			}
			if tt.wantErr && err != nil {
				if tt.errMsg != "" && !contains(err.Error(), tt.errMsg) {
					t.Errorf("validateSpec() error = %v, expected to contain %v", err.Error(), tt.errMsg)
				}
			}
		})
	}
}
