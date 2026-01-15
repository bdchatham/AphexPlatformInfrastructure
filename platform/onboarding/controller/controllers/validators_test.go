package controllers

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	platformv1alpha1 "github.com/arbiter/jenkinsx-platform/onboarding-controller/api/v1alpha1"
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
					RepoOrg:           "bdchatham",
					RepoName:          "test-repo",
					PipelineName:      "tenant-test",
					ExecutionRole: "standard",
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
					RepoOrg:           "bdchatham",
					RepoName:          "test-repo",
					PipelineName:      "tenant-test",
					ExecutionRole: "elevated",
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
					RepoOrg:           "bdchatham",
					RepoName:          "test-repo",
					PipelineName:      "tenant-test",
					ExecutionRole: "",
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
					RepoOrg:           "unapproved-org",
					RepoName:          "test-repo",
					PipelineName:      "tenant-test",
					ExecutionRole: "standard",
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
					RepoOrg:           "bdchatham",
					RepoName:          "test-repo",
					PipelineName:      "Tenant-Test",
					ExecutionRole: "standard",
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
					RepoOrg:           "bdchatham",
					RepoName:          "test-repo",
					PipelineName:      "tenant_test",
					ExecutionRole: "standard",
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
					RepoOrg:           "bdchatham",
					RepoName:          "test-repo",
					PipelineName:      "kube-system",
					ExecutionRole: "standard",
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
					RepoOrg:           "bdchatham",
					RepoName:          "test-repo",
					PipelineName:      "default",
					ExecutionRole: "standard",
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
					RepoOrg:           "bdchatham",
					RepoName:          "test-repo",
					PipelineName:      "tenant-test",
					ExecutionRole: "admin",
				},
			},
			wantErr: true,
			errMsg:  "must be 'standard' or 'elevated'",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			err := ValidateRepoBinding(tt.rb)
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
