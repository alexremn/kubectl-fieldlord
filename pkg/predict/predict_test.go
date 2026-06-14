package predict

import (
	"errors"
	"testing"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
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

func TestExtractConflicts_ConflictNoDetails(t *testing.T) {
	err := &apierrors.StatusError{ErrStatus: metav1.Status{Code: 409, Reason: metav1.StatusReasonConflict}}
	got, perr := extractConflicts(err)
	if perr != nil || got != nil {
		t.Errorf("409 nil Details -> (nil,nil); got %v,%v", got, perr)
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
