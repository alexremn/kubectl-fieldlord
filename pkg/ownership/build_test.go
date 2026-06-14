package ownership

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func mfe(mgr string, op metav1.ManagedFieldsOperationType, apiVersion, sub, raw string) metav1.ManagedFieldsEntry {
	return metav1.ManagedFieldsEntry{
		Manager: mgr, Operation: op, APIVersion: apiVersion, Subresource: sub,
		FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(raw)},
	}
}

func TestBuild_MultiOwnerAndSort(t *testing.T) {
	entries := []metav1.ManagedFieldsEntry{
		mfe("helm", metav1.ManagedFieldsOperationApply, "apps/v1", "", `{"f:spec":{"f:template":{}}}`),
		mfe("kube-controller-manager", metav1.ManagedFieldsOperationUpdate, "autoscaling/v2", "", `{"f:spec":{"f:replicas":{}}}`),
		mfe("hpa", metav1.ManagedFieldsOperationUpdate, "autoscaling/v2", "", `{"f:spec":{"f:replicas":{}}}`),
	}
	m := Build(entries)
	if len(m.Paths) != 2 {
		t.Fatalf("want 2 paths, got %d (%+v)", len(m.Paths), m.Paths)
	}
	if m.Paths[0].Path != ".spec.replicas" || m.Paths[1].Path != ".spec.template" {
		t.Fatalf("paths not sorted: %s, %s", m.Paths[0].Path, m.Paths[1].Path)
	}
	if !m.Paths[0].MultiOwner || len(m.Paths[0].Owners) != 2 {
		t.Errorf(".spec.replicas should have 2 owners and MultiOwner=true: %+v", m.Paths[0])
	}
	if m.Paths[1].MultiOwner {
		t.Errorf(".spec.template should be single-owner")
	}
}

func TestBuild_SameManagerMultipleEntriesStayDistinct(t *testing.T) {
	entries := []metav1.ManagedFieldsEntry{
		mfe("kubectl", metav1.ManagedFieldsOperationApply, "apps/v1", "", `{"f:spec":{"f:replicas":{}}}`),
		mfe("kubectl", metav1.ManagedFieldsOperationUpdate, "apps/v1", "", `{"f:spec":{"f:replicas":{}}}`),
		mfe("kubectl", metav1.ManagedFieldsOperationUpdate, "apps/v1", "status", `{"f:spec":{"f:replicas":{}}}`),
	}
	m := Build(entries)
	if len(m.Paths) != 1 {
		t.Fatalf("want 1 path, got %d", len(m.Paths))
	}
	if len(m.Paths[0].Owners) != 3 {
		t.Errorf("expected 3 distinct kubectl owners, got %d: %+v", len(m.Paths[0].Owners), m.Paths[0].Owners)
	}
	if m.Paths[0].MultiOwner {
		t.Errorf("same manager across entries must not count as MultiOwner")
	}
}

func TestBuild_AtomicFlagFromIndex(t *testing.T) {
	m := Build([]metav1.ManagedFieldsEntry{
		mfe("kubelet", metav1.ManagedFieldsOperationUpdate, "v1", "status",
			`{"f:status":{"f:conditions":{"i:0":{"f:type":{}}}}}`),
	})
	var found bool
	for _, p := range m.Paths {
		if p.Path == ".status.conditions[0].type" {
			found = true
			if !p.Atomic {
				t.Errorf("indexed path should be flagged atomic: %+v", p)
			}
		}
	}
	if !found {
		t.Fatalf("expected indexed path in model: %+v", m.Paths)
	}
}

func TestIsAtomicPath(t *testing.T) {
	cases := map[string]bool{
		".status.conditions[0].type":         true,
		".spec.replicas":                     false,
		`.spec.containers[name="app"].image`: false,
		`.spec.finalizers[="keep"]`:          false,
		`.spec.ports[=80]`:                   false,
		`.spec.things[id=5]`:                 false,
	}
	for path, want := range cases {
		if got := isAtomicPath(path); got != want {
			t.Errorf("isAtomicPath(%q) = %v, want %v", path, got, want)
		}
	}
}
