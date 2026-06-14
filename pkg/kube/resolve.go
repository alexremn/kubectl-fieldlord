package kube

import (
	"k8s.io/cli-runtime/pkg/resource"
)

// Resolve turns kubectl-style resource refs (e.g. "deploy/api") into *resource.Info
// objects. Fetched objects include managedFields (the builder does not strip them).
func Resolve(getter resource.RESTClientGetter, namespace string, args []string) ([]*resource.Info, error) {
	r := resource.NewBuilder(getter).
		Unstructured().
		NamespaceParam(namespace).DefaultNamespace().
		ResourceTypeOrNameArgs(true, args...).
		ContinueOnError().
		Latest().
		Flatten().
		Do()
	if err := r.Err(); err != nil {
		return nil, err
	}
	return r.Infos()
}
