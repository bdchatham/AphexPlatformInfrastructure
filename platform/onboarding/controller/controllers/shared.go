package controllers

import (
	"k8s.io/apimachinery/pkg/runtime"
	"sigs.k8s.io/controller-runtime/pkg/client"
	"sigs.k8s.io/controller-runtime/pkg/controller/controllerutil"

	platformv1alpha1 "github.com/arbiter/jenkinsx-platform/onboarding-controller/api/v1alpha1"
)

// Shared utility functions used by multiple controllers

// setOwnerReference sets the RepoBinding as the owner of a resource
func setOwnerReference(rb *platformv1alpha1.RepoBinding, obj client.Object, scheme *runtime.Scheme) error {
	return controllerutil.SetControllerReference(rb, obj, scheme)
}

// stringPtr returns a pointer to a string
func stringPtr(s string) *string {
	return &s
}
