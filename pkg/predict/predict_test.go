package predict

import (
	"errors"
	"slices"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	dynamicfake "k8s.io/client-go/dynamic/fake"
	clienttesting "k8s.io/client-go/testing"
)

func conflict(causes ...metav1.StatusCause) error {
	return &apierrors.StatusError{ErrStatus: metav1.Status{
		Status: "Failure", Code: 409, Reason: metav1.StatusReasonConflict,
		Details: &metav1.StatusDetails{Causes: causes},
	}}
}

func cause(field, msg string) metav1.StatusCause {
	return metav1.StatusCause{Type: metav1.CauseTypeFieldManagerConflict, Field: field, Message: msg}
}

func TestExtractConflicts_ParsesFieldsAndManagers(t *testing.T) {
	err := conflict(
		cause(".spec.replicas", `conflict with "kube-controller-manager" using autoscaling/v2`),
		cause(`.spec.template.spec.containers[name="app"].image`, `conflict with "helm"`),
	)
	got, perr := extractConflicts(err)
	if perr != nil {
		t.Fatalf("extractConflicts error = %v", perr)
	}
	if len(got) != 2 {
		t.Fatalf("want 2, got %d: %+v", len(got), got)
	}
	if got[0].Field != ".spec.replicas" || got[0].Manager != "kube-controller-manager" {
		t.Errorf("got[0] = %+v", got[0])
	}
	if got[1].Manager != "helm" {
		t.Errorf("got[1].Manager = %q", got[1].Manager)
	}
}

func TestExtractConflicts_NoErrorIsEmpty(t *testing.T) {
	got, err := extractConflicts(nil)
	if err != nil || got != nil {
		t.Errorf("nil -> (nil,nil); got %v,%v", got, err)
	}
}

func TestExtractConflicts_NonConflictPropagates(t *testing.T) {
	if _, err := extractConflicts(errors.New("rbac forbidden")); err == nil {
		t.Errorf("non-409 must propagate")
	}
}

// A 409 without field-manager causes is not an SSA ownership conflict (e.g. a
// stale metadata.resourceVersion): the apply would fail outright, so it must
// propagate rather than read as an empty (all-clear) conflict set.
func TestExtractConflicts_CauselessConflictPropagates(t *testing.T) {
	cases := map[string]error{
		"nil details": &apierrors.StatusError{ErrStatus: metav1.Status{Code: 409, Reason: metav1.StatusReasonConflict}},
		"no causes":   conflict(),
		"optimistic lock": apierrors.NewConflict(schema.GroupResource{Group: "apps", Resource: "deployments"}, "api",
			errors.New("the object has been modified; please apply your changes to the latest version and try again")),
		"other cause type": conflict(metav1.StatusCause{Type: metav1.CauseTypeFieldValueInvalid, Field: ".spec"}),
	}
	for name, in := range cases {
		got, err := extractConflicts(in)
		if !errors.Is(err, in) || got != nil {
			t.Errorf("%s: want (nil, input error); got %v, %v", name, got, err)
		}
	}
}

func TestExtractConflicts_SkipsNonFieldManagerCauses(t *testing.T) {
	got, err := extractConflicts(conflict(
		metav1.StatusCause{Type: metav1.CauseTypeFieldValueInvalid, Field: ".spec"},
		cause(".spec.replicas", `conflict with "hpa"`),
	))
	if err != nil {
		t.Fatalf("extractConflicts error = %v", err)
	}
	if len(got) != 1 || got[0] != (ConflictPath{Field: ".spec.replicas", Manager: "hpa"}) {
		t.Errorf("got %+v", got)
	}
}

func TestExtractConflicts_SortsByFieldThenManager(t *testing.T) {
	got, err := extractConflicts(conflict(
		cause(".spec.replicas", `conflict with "b"`),
		cause(".metadata.labels.x", `conflict with "z"`),
		cause(".spec.replicas", `conflict with "a"`),
	))
	if err != nil {
		t.Fatalf("extractConflicts error = %v", err)
	}
	want := []ConflictPath{{".metadata.labels.x", "z"}, {".spec.replicas", "a"}, {".spec.replicas", "b"}}
	if !slices.Equal(got, want) {
		t.Errorf("got %+v, want %+v", got, want)
	}
}

// Probe's safety contract: a NON-forced apply under DryRun=All, so it can
// neither persist nor take ownership away from anyone.
func TestProbe_SendsNonForcedDryRunApply(t *testing.T) {
	dyn := dynamicfake.NewSimpleDynamicClient(runtime.NewScheme())
	var got clienttesting.PatchActionImpl
	dyn.PrependReactor("patch", "deployments", func(a clienttesting.Action) (bool, runtime.Object, error) {
		got = a.(clienttesting.PatchActionImpl)
		return true, nil, conflict(cause(".spec.replicas", `conflict with "hpa"`))
	})
	gvr := schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}

	conflicts, err := Probe(t.Context(), dyn.Resource(gvr).Namespace("default"), "api", []byte(`{"kind":"Deployment"}`), "helm")
	if err != nil {
		t.Fatalf("Probe error = %v", err)
	}
	if want := []ConflictPath{{".spec.replicas", "hpa"}}; !slices.Equal(conflicts, want) {
		t.Errorf("conflicts = %+v, want %+v", conflicts, want)
	}
	if got.GetPatchType() != types.ApplyPatchType || got.GetName() != "api" || got.GetNamespace() != "default" {
		t.Errorf("patch = type %q %s/%s, want apply default/api", got.GetPatchType(), got.GetNamespace(), got.GetName())
	}
	opts := got.GetPatchOptions()
	if opts.FieldManager != "helm" {
		t.Errorf("FieldManager = %q, want helm", opts.FieldManager)
	}
	if !slices.Equal(opts.DryRun, []string{metav1.DryRunAll}) {
		t.Errorf("DryRun = %v, want [All]", opts.DryRun)
	}
	if opts.Force == nil || *opts.Force {
		t.Errorf("Force = %v, want explicit false", opts.Force)
	}
}

func TestParseManager(t *testing.T) {
	cases := map[string]string{
		`conflict with "helm"`:                                  "helm",
		`conflict with "kube-controller-manager" using apps/v1`: "kube-controller-manager",
		`conflict with "argocd-controller" with subresource "status" using apps/v1 at 2024-01-01T00:00:00Z`: "argocd-controller",
		`something unexpected`: "",
	}
	for msg, want := range cases {
		if got := parseManager(msg); got != want {
			t.Errorf("parseManager(%q) = %q, want %q", msg, got, want)
		}
	}
}
