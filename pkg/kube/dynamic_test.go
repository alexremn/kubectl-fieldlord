package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/api/meta"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	"k8s.io/client-go/kubernetes/scheme"
)

func TestGVRFromInfo(t *testing.T) {
	info := &resource.Info{Mapping: &meta.RESTMapping{
		Resource: schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"},
	}}
	gvr, err := GVRFromInfo(info)
	if err != nil || gvr.Resource != "deployments" {
		t.Fatalf("gvr=%+v err=%v", gvr, err)
	}
}

func TestGVRFromInfo_NilMapping(t *testing.T) {
	if _, err := GVRFromInfo(&resource.Info{}); err == nil {
		t.Errorf("nil mapping must error")
	}
}

func TestResourceInterface_NamespacedVsCluster(t *testing.T) {
	dc := dynamicfake.NewSimpleDynamicClient(scheme.Scheme)
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	if resourceInterface(dc, gvr, meta.RESTScopeNameNamespace, "default") == nil {
		t.Error("namespaced resourceInterface nil")
	}
	if resourceInterface(dc, gvr, meta.RESTScopeNameRoot, "") == nil {
		t.Error("cluster resourceInterface nil")
	}
}

func TestDynamicClient_BadConfigErrors(t *testing.T) {
	cf := genericclioptions.NewConfigFlags(false)
	bad := "/nonexistent/this/path/kubeconfig"
	cf.KubeConfig = &bad
	if _, err := DynamicClient(cf); err == nil {
		t.Errorf("expected error from bad kubeconfig")
	}
}
