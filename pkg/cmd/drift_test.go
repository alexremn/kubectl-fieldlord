package cmd

import (
	"bytes"
	"encoding/json"
	"errors"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"

	"k8s.io/cli-runtime/pkg/genericiooptions"
)

func driftMF() []metav1.ManagedFieldsEntry {
	return []metav1.ManagedFieldsEntry{
		{Manager: "helm", Operation: metav1.ManagedFieldsOperationApply, APIVersion: "apps/v1",
			FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:template":{}}}`)}},
		{Manager: "hpa", Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "autoscaling/v2",
			FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)}},
	}
}

func TestRenderDrift_ExitsTwoOnAttributedDrift(t *testing.T) {
	u := deploy("api", driftMF())
	var out bytes.Buffer
	err := renderDrift(&out, "table", true, u, "helm", "v1.34.2", "full")
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected ExitError code 2, got %v", err)
	}
}

func TestRenderDrift_NoFindings_NoError(t *testing.T) {
	u := deploy("api", []metav1.ManagedFieldsEntry{
		{Manager: "helm", Operation: metav1.ManagedFieldsOperationApply, APIVersion: "apps/v1",
			FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)}},
	})
	var out bytes.Buffer
	if err := renderDrift(&out, "table", true, u, "helm", "v1.34.2", "full"); err != nil {
		t.Fatalf("expected no error when no drift, got %v", err)
	}
}

func TestRunDrift_AggregatesExitTwo(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "table", noColor: true, skipVersionCheck: true},
		resolve: fakeResolve(deploy("a", driftMF()), deploy("b", driftMF())),
	}
	err := runDrift(o, "helm", streams)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("expected aggregate ExitError code 2 across objects, got %v", err)
	}
}

func TestRunDrift_JSONNoFindings(t *testing.T) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	clean := deploy("api", []metav1.ManagedFieldsEntry{
		{Manager: "helm", Operation: metav1.ManagedFieldsOperationApply, APIVersion: "apps/v1",
			FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)}},
	})
	o := &cmdOptions{
		g:       &globalOptions{output: "json", skipVersionCheck: true},
		resolve: fakeResolve(clean),
	}
	if err := runDrift(o, "helm", streams); err != nil {
		t.Fatalf("clean object should not error: %v", err)
	}
	var env map[string]any
	if err := json.Unmarshal(out.Bytes(), &env); err != nil {
		t.Fatalf("drift json must be a valid object: %v\n%s", err, out.String())
	}
	if env["command"] != "drift" {
		t.Errorf("expected command=drift, got %v", env["command"])
	}
}
