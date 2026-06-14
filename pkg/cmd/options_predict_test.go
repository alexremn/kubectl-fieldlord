package cmd

import (
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
	"github.com/alexremn/kubectl-fieldlord/pkg/predict"
)

func TestSSASupported(t *testing.T) {
	for tier, want := range map[string]bool{"full": true, "": true, "unknown": true, "best-effort": false, "unsupported": false} {
		if got := ssaSupported(tier); got != want {
			t.Errorf("ssaSupported(%q)=%v want %v", tier, got, want)
		}
	}
}

func TestHasApplyManager(t *testing.T) {
	mf := []metav1.ManagedFieldsEntry{
		{Manager: "helm", Operation: metav1.ManagedFieldsOperationApply},
		{Manager: "hpa", Operation: metav1.ManagedFieldsOperationUpdate},
	}
	if !hasApplyManager(mf, "helm") {
		t.Error("helm should be an apply manager")
	}
	if hasApplyManager(mf, "hpa") {
		t.Error("hpa is Update, not an apply manager")
	}
	if hasApplyManager(mf, "ghost") {
		t.Error("ghost absent")
	}
}

func TestEnrichConflicts(t *testing.T) {
	model := ownership.Model{Paths: []ownership.OwnedPath{{
		Path:   ".spec.replicas",
		Owners: []ownership.Owner{{Manager: "hpa", Operation: ownership.OperationUpdate, APIVersion: "autoscaling/v2"}},
	}}}
	got := enrichConflicts([]predict.ConflictPath{{Field: ".spec.replicas", Manager: "hpa"}}, model, true)
	if len(got) != 1 || got[0].Path != ".spec.replicas" || !got[0].LowConfidence {
		t.Fatalf("got %+v", got)
	}
	if got[0].CurrentOwner == nil || got[0].CurrentOwner.Operation != ownership.OperationUpdate {
		t.Errorf("currentOwner not enriched from model: %+v", got[0].CurrentOwner)
	}
	// path absent from model -> fallback to Manager-only owner
	got2 := enrichConflicts([]predict.ConflictPath{{Field: ".spec.x", Manager: "kubectl"}}, model, false)
	if got2[0].CurrentOwner == nil || got2[0].CurrentOwner.Manager != "kubectl" {
		t.Errorf("fallback owner missing: %+v", got2[0].CurrentOwner)
	}
}
