/*
Copyright 2019 The Knative Authors

Licensed under the Apache License, Version 2.0 (the "License");
you may not use this file except in compliance with the License.
You may obtain a copy of the License at

    http://www.apache.org/licenses/LICENSE-2.0

Unless required by applicable law or agreed to in writing, software
distributed under the License is distributed on an "AS IS" BASIS,
WITHOUT WARRANTIES OR CONDITIONS OF ANY KIND, either express or implied.
See the License for the specific language governing permissions and
limitations under the License.
*/

// Package common contains values that are common across client and server,
// to allow the client to use well known names to talk to the server
package common

import (
	"log"
	"reflect"

	appsv1 "k8s.io/api/apps/v1"
	authenticationv1 "k8s.io/api/authentication/v1"
	batchv1 "k8s.io/api/batch/v1"
	corev1 "k8s.io/api/core/v1"
	networkingv1 "k8s.io/api/networking/v1"
	rbacv1 "k8s.io/api/rbac/v1"
	schedulingv1 "k8s.io/api/scheduling/v1"
	storagev1 "k8s.io/api/storage/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
)

var (
	// ResourceMap is a hardcoded map of GVK to reflect.Type
	ResourceMap = buildResourceMap()
)

// buildResourceMap returns a map of GVK to reflect.Type for all kubernetes v1
// groups, except those resources that are explicitly removed at the end
func buildResourceMap() map[schema.GroupVersionKind]reflect.Type {
	gvkToType := make(map[schema.GroupVersionKind]reflect.Type)
	// Should I be making my own schemes or is there some place I can find pre-built schemes?
	schemeBuilders := []runtime.SchemeBuilder{
		appsv1.SchemeBuilder,
		authenticationv1.SchemeBuilder,
		batchv1.SchemeBuilder,
		corev1.SchemeBuilder,
		rbacv1.SchemeBuilder,
		networkingv1.SchemeBuilder,
		schedulingv1.SchemeBuilder,
		storagev1.SchemeBuilder,
	}
	for _, sb := range schemeBuilders {
		s := runtime.NewScheme()
		err := sb.AddToScheme(s)
		if err != nil {
			log.Fatalf("couldn't build scheme: %v", err)
		}
		schemeGvkToType := s.AllKnownTypes()
		for gvk, resourceType := range schemeGvkToType {
			gvkToType[gvk] = resourceType
		}
	}
	// We're going to ignore corev1.Event because Kubernetes conformance tests
	// are not allowed to rely on it, it's nothing but optional fields with no
	// guarantee of delivery
	delete(gvkToType, corev1.SchemeGroupVersion.WithKind("Event"))
	return gvkToType
}

const (
	// WebhookNamespace is a hardcoded value for the namespace that holds all
	// resources involved in running k8s-api-coverage-server; MUST match how
	// this is actually deployed
	WebhookNamespace = "k8s-api-coverage"

	// CommonComponentName is a hardcoded value for the name or prefix used by
	// all resources involved in running k8s-api-coverage-server; MUST match
	// how this is actually deployed
	CommonComponentName = "apicoverage-webhook"
)
