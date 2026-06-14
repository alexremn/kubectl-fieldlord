package ownership

import (
	"reflect"
	"sort"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

func entry(raw string) metav1.ManagedFieldsEntry {
	return metav1.ManagedFieldsEntry{
		Manager:    "helm",
		Operation:  metav1.ManagedFieldsOperationApply,
		APIVersion: "apps/v1",
		FieldsType: "FieldsV1",
		FieldsV1:   &metav1.FieldsV1{Raw: []byte(raw)},
	}
}

func TestDecodeEntry_LeafPaths(t *testing.T) {
	raw := `{"f:spec":{".":{},"f:replicas":{},` +
		`"f:template":{"f:spec":{"f:containers":{"k:{\"name\":\"app\"}":{"f:image":{}}}}},` +
		`"f:finalizers":{"v:\"keep\"":{}}}}`
	owner, paths, warnings, err := decodeEntry(entry(raw))
	if err != nil {
		t.Fatalf("decodeEntry error = %v", err)
	}
	if owner.Manager != "helm" || owner.Operation != OperationApply || owner.APIVersion != "apps/v1" {
		t.Errorf("owner mismatch: %+v", owner)
	}
	if len(warnings) != 0 {
		t.Errorf("unexpected warnings: %v", warnings)
	}
	sort.Strings(paths)
	want := []string{
		`.spec.finalizers[="keep"]`,
		".spec.replicas",
		`.spec.template.spec.containers[name="app"].image`,
	}
	if !reflect.DeepEqual(paths, want) {
		t.Errorf("paths = %#v, want %#v", paths, want)
	}
}

func TestDecodeEntry_ZeroLeafWarns(t *testing.T) {
	_, paths, warnings, err := decodeEntry(entry(`{"z:bogus":{}}`))
	if err != nil {
		t.Fatalf("decodeEntry error = %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("expected zero leaf paths, got %v", paths)
	}
	if len(warnings) == 0 {
		t.Errorf("expected a warning for an entry that decoded to no paths")
	}
}

func TestDecodeEntry_NilFieldsV1(t *testing.T) {
	e := metav1.ManagedFieldsEntry{Manager: "x", Operation: metav1.ManagedFieldsOperationUpdate}
	_, paths, _, err := decodeEntry(e)
	if err != nil || len(paths) != 0 {
		t.Errorf("nil FieldsV1 should yield no paths, no error; got paths=%v err=%v", paths, err)
	}
}
