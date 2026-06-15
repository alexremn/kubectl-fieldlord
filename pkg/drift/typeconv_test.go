package drift

import (
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
)

func u(c map[string]any) *unstructured.Unstructured { return &unstructured.Unstructured{Object: c} }

func TestDiff_DeducedDetectsScalarModified(t *testing.T) {
	tc := managedfields.NewDeducedTypeConverter()
	live := u(map[string]any{"apiVersion": "v1", "kind": "X", "spec": map[string]any{"replicas": int64(3)}})
	desired := u(map[string]any{"apiVersion": "v1", "kind": "X", "spec": map[string]any{"replicas": int64(9)}})
	cmp, _, err := Diff(desired, live, tc, &fieldpath.Set{})
	if err != nil {
		t.Fatal(err)
	}
	_, modified, _ := CollectChanged(cmp)
	if !containsSub(modified, "replicas") {
		t.Errorf("expected .spec.replicas modified; got %v", modified)
	}
}

func TestDiff_RemovedIntersectedWithOwned(t *testing.T) {
	tc := managedfields.NewDeducedTypeConverter()
	live := u(map[string]any{"apiVersion": "v1", "kind": "X", "spec": map[string]any{"paused": true, "foo": "bar"}})
	desired := u(map[string]any{"apiVersion": "v1", "kind": "X", "spec": map[string]any{}})
	mf := []metav1.ManagedFieldsEntry{{Manager: "helm", Operation: metav1.ManagedFieldsOperationApply,
		FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:paused":{}}}`)}}}
	owned := ManagerOwnedSet(mf, "helm")
	cmp, _, err := Diff(desired, live, tc, owned)
	if err != nil {
		t.Fatal(err)
	}
	_, _, removed := CollectChanged(cmp)
	if !containsSub(removed, "paused") {
		t.Errorf("owned removed field .spec.paused must remain; removed=%v", removed)
	}
	if containsSub(removed, "foo") {
		t.Errorf("un-owned removed field .spec.foo must be suppressed; removed=%v", removed)
	}
}

func TestDiff_ModifiedNotSuppressedByOwned(t *testing.T) {
	// Q2 regression guard: a Modified field must survive a non-empty owned-set
	// intersection (we only intersect Removed, never Modified).
	tc := managedfields.NewDeducedTypeConverter()
	live := u(map[string]any{"apiVersion": "v1", "kind": "X", "spec": map[string]any{"replicas": int64(1)}})
	desired := u(map[string]any{"apiVersion": "v1", "kind": "X", "spec": map[string]any{"replicas": int64(2)}})
	owned := ManagerOwnedSet([]metav1.ManagedFieldsEntry{{Manager: "z", Operation: metav1.ManagedFieldsOperationApply,
		FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:metadata":{"f:name":{}}}`)}}}, "z")
	cmp, _, err := Diff(desired, live, tc, owned)
	if err != nil {
		t.Fatal(err)
	}
	_, modified, _ := CollectChanged(cmp)
	if !containsSub(modified, "replicas") {
		t.Errorf("Modified must NOT be suppressed by owned-set; got %v", modified)
	}
}

func TestManagerOwnedSet_SkipsSubresourceNilOther(t *testing.T) {
	mf := []metav1.ManagedFieldsEntry{
		{Manager: "helm", Operation: metav1.ManagedFieldsOperationApply, FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)}},
		{Manager: "helm", Subresource: "status", FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:status":{"f:x":{}}}`)}},
		{Manager: "helm", FieldsV1: nil},
		{Manager: "helm", FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{bad json`)}},
		{Manager: "other", FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:other":{}}}`)}},
	}
	set := ManagerOwnedSet(mf, "helm")
	if set.Empty() {
		t.Fatal("expected non-empty owned set")
	}
	var paths []string
	set.Leaves().Iterate(func(p fieldpath.Path) { paths = append(paths, p.String()) })
	for _, p := range paths {
		if strings.Contains(p, "status") || strings.Contains(p, "other") {
			t.Errorf("subresource/other-manager field leaked: %v", paths)
		}
	}
}

func TestScrub_RemovesServerManagedFields(t *testing.T) {
	obj := u(map[string]any{
		"apiVersion": "v1", "kind": "X",
		"metadata": map[string]any{
			"name": "a", "resourceVersion": "9", "uid": "z", "creationTimestamp": "t",
			"generation": int64(2), "managedFields": []any{map[string]any{"manager": "m"}},
			"annotations": map[string]any{"kubectl.kubernetes.io/last-applied-configuration": "{}", "keep": "v"},
		},
		"spec":   map[string]any{"replicas": int64(1)},
		"status": map[string]any{"x": int64(1)},
	})
	s := Scrub(obj, false)
	md, _, _ := unstructured.NestedMap(s.Object, "metadata")
	for _, k := range []string{"resourceVersion", "uid", "creationTimestamp", "generation", "managedFields"} {
		if _, ok := md[k]; ok {
			t.Errorf("metadata.%s not scrubbed", k)
		}
	}
	if _, ok := s.Object["status"]; ok {
		t.Error("status not scrubbed (includeStatus=false)")
	}
	if md["name"] != "a" {
		t.Error("metadata.name must survive")
	}
	// includeStatus=true keeps status
	if _, ok := Scrub(obj, true).Object["status"]; !ok {
		t.Error("status must survive includeStatus=true")
	}
}

func TestIsNoCorrespondingType(t *testing.T) {
	if !isNoCorrespondingType(errStr("no corresponding type for /v1, Kind=Foo")) {
		t.Error("should match")
	}
	if isNoCorrespondingType(errStr("other")) || isNoCorrespondingType(nil) {
		t.Error("should not match")
	}
}

func containsSub(ss []string, sub string) bool {
	for _, s := range ss {
		if strings.Contains(s, sub) {
			return true
		}
	}
	return false
}

type errStr string

func (e errStr) Error() string { return string(e) }
