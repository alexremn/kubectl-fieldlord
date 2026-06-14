package drift

import (
	"testing"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

func own(mgr string, op ownership.Operation, sub, tm string) ownership.Owner {
	return ownership.Owner{Manager: mgr, Operation: op, Subresource: sub, Time: tm}
}

func TestNative_ExpectManager_FlagsForeignFields(t *testing.T) {
	m := ownership.Model{Paths: []ownership.OwnedPath{
		{Path: ".spec.template", Owners: []ownership.Owner{own("helm", ownership.OperationApply, "", "")}},
		{Path: ".spec.replicas", Owners: []ownership.Owner{own("hpa", ownership.OperationUpdate, "", "")}},
	}}
	findings, err := Native(m, "helm")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 1 || findings[0].Path != ".spec.replicas" {
		t.Fatalf("want 1 finding on .spec.replicas, got %+v", findings)
	}
	if findings[0].ActualOwner == nil || findings[0].ActualOwner.Manager != "hpa" || !findings[0].Attributed {
		t.Errorf("expected attributed drift to hpa: %+v", findings[0])
	}
}

func TestNative_SubresourceFieldsNotDrift(t *testing.T) {
	m := ownership.Model{Paths: []ownership.OwnedPath{
		{Path: ".spec.replicas", Owners: []ownership.Owner{own("helm", ownership.OperationApply, "", "")}},
		{Path: ".status.readyReplicas", Owners: []ownership.Owner{own("kubelet", ownership.OperationUpdate, "status", "")}},
	}}
	findings, err := Native(m, "helm")
	if err != nil {
		t.Fatal(err)
	}
	if len(findings) != 0 {
		t.Errorf("subresource-owned fields must not be drift: %+v", findings)
	}
}

func TestNative_InferPrimary_MostLeavesThenTime(t *testing.T) {
	m := ownership.Model{Paths: []ownership.OwnedPath{
		{Path: ".spec.a", Owners: []ownership.Owner{own("helm", ownership.OperationApply, "", "2026-01-01T00:00:00Z")}},
		{Path: ".spec.b", Owners: []ownership.Owner{own("argocd", ownership.OperationApply, "", "2026-02-01T00:00:00Z")}},
		{Path: ".spec.replicas", Owners: []ownership.Owner{own("hpa", ownership.OperationUpdate, "", "")}},
	}}
	primary, ok := inferPrimary(m)
	if !ok || primary != "argocd" {
		t.Errorf("inferPrimary = %q,%v; want argocd,true", primary, ok)
	}
}

func TestNative_NoApplyManager_Errors(t *testing.T) {
	m := ownership.Model{Paths: []ownership.OwnedPath{
		{Path: ".spec.replicas", Owners: []ownership.Owner{own("hpa", ownership.OperationUpdate, "", "")}},
	}}
	if _, err := Native(m, ""); err == nil {
		t.Errorf("expected error when no Apply manager exists and --expect-manager is unset")
	}
}

func TestNative_InferPrimary_MostLeavesWins(t *testing.T) {
	// helm owns 2 leaves, argocd owns 1 -> helm wins on count regardless of order.
	m := ownership.Model{Paths: []ownership.OwnedPath{
		{Path: ".spec.a", Owners: []ownership.Owner{own("helm", ownership.OperationApply, "", "")}},
		{Path: ".spec.b", Owners: []ownership.Owner{own("helm", ownership.OperationApply, "", "")}},
		{Path: ".spec.c", Owners: []ownership.Owner{own("argocd", ownership.OperationApply, "", "")}},
	}}
	primary, ok := inferPrimary(m)
	if !ok || primary != "helm" {
		t.Errorf("inferPrimary = %q,%v; want helm,true (2 leaves > 1)", primary, ok)
	}
}

func TestNative_InferPrimary_NameTieBreak(t *testing.T) {
	// equal count, equal (empty) time -> lexicographically smallest name wins.
	m := ownership.Model{Paths: []ownership.OwnedPath{
		{Path: ".spec.a", Owners: []ownership.Owner{own("zeta", ownership.OperationApply, "", "")}},
		{Path: ".spec.b", Owners: []ownership.Owner{own("alpha", ownership.OperationApply, "", "")}},
	}}
	primary, ok := inferPrimary(m)
	if !ok || primary != "alpha" {
		t.Errorf("inferPrimary = %q,%v; want alpha,true (name tie-break)", primary, ok)
	}
}
