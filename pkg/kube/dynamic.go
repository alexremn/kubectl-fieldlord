package kube

import (
	"fmt"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"k8s.io/client-go/dynamic"
)

// DynamicClient builds a dynamic client from the plugin's config flags.
func DynamicClient(getter genericclioptions.RESTClientGetter) (dynamic.Interface, error) {
	cfg, err := getter.ToRESTConfig()
	if err != nil {
		return nil, fmt.Errorf("building REST config: %w", err)
	}
	dc, err := dynamic.NewForConfig(cfg)
	if err != nil {
		return nil, fmt.Errorf("building dynamic client: %w", err)
	}
	return dc, nil
}

// GVRFromInfo extracts the GroupVersionResource from a resolved resource.Info.
func GVRFromInfo(info *resource.Info) (schema.GroupVersionResource, error) {
	if info == nil || info.Mapping == nil {
		return schema.GroupVersionResource{}, fmt.Errorf("resource has no REST mapping")
	}
	return info.Mapping.Resource, nil
}

// resourceInterface returns a namespaced or cluster-scoped dynamic interface based
// on the resource's REST scope (not on whether namespace is empty).
func resourceInterface(dyn dynamic.Interface, gvr schema.GroupVersionResource, scope meta.RESTScopeName, namespace string) dynamic.ResourceInterface {
	if scope == meta.RESTScopeNameNamespace {
		return dyn.Resource(gvr).Namespace(namespace)
	}
	return dyn.Resource(gvr)
}

// ResourceInterfaceForInfo builds the dynamic ResourceInterface for a resolved object.
func ResourceInterfaceForInfo(dyn dynamic.Interface, info *resource.Info) (dynamic.ResourceInterface, error) {
	gvr, err := GVRFromInfo(info)
	if err != nil {
		return nil, err
	}
	return resourceInterface(dyn, gvr, info.Mapping.Scope.Name(), info.Namespace), nil
}
