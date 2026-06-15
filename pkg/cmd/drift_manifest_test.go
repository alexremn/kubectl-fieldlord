package cmd

import (
	"encoding/json"
	"errors"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v6/typed"

	"github.com/alexremn/kubectl-fieldlord/pkg/drift"
)

// cannedDiff builds a real *typed.Comparison via the deduced converter so
// CollectChanged yields genuine paths, then returns it from an injected diffFunc.
func cannedDiff(t *testing.T, liveObj, desiredObj map[string]any, schemaUsed bool) diffFunc {
	t.Helper()
	tc := managedfields.NewDeducedTypeConverter()
	cmp, _, err := drift.Diff(&unstructured.Unstructured{Object: desiredObj}, &unstructured.Unstructured{Object: liveObj}, tc, &fieldpath.Set{})
	if err != nil {
		t.Fatalf("building canned comparison: %v", err)
	}
	return func(_, _ *unstructured.Unstructured, _ *fieldpath.Set) (*typed.Comparison, bool, error) {
		return cmp, schemaUsed, nil
	}
}

func deployWithReplicas(name, owner string, replicas int64) *unstructured.Unstructured {
	d := deploy(name, []metav1.ManagedFieldsEntry{{Manager: owner, Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "apps/v1",
		FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)}}})
	_ = unstructured.SetNestedField(d.Object, replicas, "spec", "replicas")
	return d
}

func TestRunDriftManifest_ConflictExitsTwo(t *testing.T) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	live := deployWithReplicas("api", "hpa", 3)
	o := &cmdOptions{g: &globalOptions{output: "json", skipVersionCheck: true}, resolve: fakeResolve(live)}
	o.diff = cannedDiff(t,
		map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "spec": map[string]any{"replicas": int64(3)}},
		map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "spec": map[string]any{"replicas": int64(9)}}, true)
	err := runDriftManifest(o, []byte(`{"spec":{"replicas":9}}`), "helm", false, streams)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("want ExitError 2, got %v", err)
	}
	var env map[string]any
	if jerr := json.Unmarshal(out.Bytes(), &env); jerr != nil {
		t.Fatalf("invalid json: %v\n%s", jerr, out.String())
	}
	if env["command"] != "drift" {
		t.Errorf("command=%v", env["command"])
	}
}

func TestRunDriftManifest_SelfChangeExitsZero(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	live := deployWithReplicas("api", "helm", 3) // owned by helm
	o := &cmdOptions{g: &globalOptions{output: "table", noColor: true, skipVersionCheck: true}, resolve: fakeResolve(live)}
	o.diff = cannedDiff(t,
		map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "spec": map[string]any{"replicas": int64(3)}},
		map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "spec": map[string]any{"replicas": int64(9)}}, true)
	if err := runDriftManifest(o, []byte(`{"spec":{"replicas":9}}`), "helm", false, streams); err != nil {
		t.Fatalf("self-change must not gate, got %v", err)
	}
}

func TestRunDriftManifest_EmptyExpectManagerInformational(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	live := deployWithReplicas("api", "hpa", 3)
	o := &cmdOptions{g: &globalOptions{output: "table", noColor: true, skipVersionCheck: true}, resolve: fakeResolve(live)}
	o.diff = cannedDiff(t,
		map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "spec": map[string]any{"replicas": int64(3)}},
		map[string]any{"apiVersion": "apps/v1", "kind": "Deployment", "spec": map[string]any{"replicas": int64(9)}}, true)
	if err := runDriftManifest(o, []byte(`{"spec":{"replicas":9}}`), "", false, streams); err != nil {
		t.Fatalf("empty expect-manager must be informational (exit 0), got %v", err)
	}
}

func TestRunDriftManifest_GarbageYAMLExitsOne(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{g: &globalOptions{output: "table", skipVersionCheck: true}, resolve: fakeResolve(deploy("api", nil)),
		diff: func(_, _ *unstructured.Unstructured, _ *fieldpath.Set) (*typed.Comparison, bool, error) {
			return nil, true, nil
		}}
	if err := runDriftManifest(o, []byte("{not: valid: yaml: ["), "helm", false, streams); err == nil || !strings.Contains(err.Error(), "decoding desired manifest") {
		t.Errorf("garbage yaml must error 'decoding desired manifest'; got %v", err)
	}
}

func TestRunDriftManifest_MultipleResourcesExitsOne(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{g: &globalOptions{output: "table", skipVersionCheck: true}, resolve: fakeResolve(deploy("a", nil), deploy("b", nil)),
		diff: func(_, _ *unstructured.Unstructured, _ *fieldpath.Set) (*typed.Comparison, bool, error) {
			return nil, true, nil
		}}
	if err := runDriftManifest(o, []byte(`{}`), "helm", false, streams); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("multiple resources must error 'exactly one'; got %v", err)
	}
}

func TestConflictCount(t *testing.T) {
	f := []drift.Finding{{Conflict: true}, {Conflict: false}, {Conflict: true}}
	if conflictCount(f) != 2 {
		t.Errorf("conflictCount=%d want 2", conflictCount(f))
	}
}

func TestNativeDriftJSON_HasNoManifestKeys(t *testing.T) {
	// omitempty regression guard: native-mode finding JSON must not contain
	// change/conflict/granularity keys.
	f := drift.Finding{Path: ".spec.replicas", ExpectedManager: "helm", Attributed: true}
	b, _ := json.Marshal(f)
	for _, k := range []string{"change", "conflict", "granularity"} {
		if strings.Contains(string(b), k) {
			t.Errorf("native finding JSON must omit %q: %s", k, b)
		}
	}
}
