package cmd

import (
	"bytes"
	"encoding/json"
	"fmt"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
	"github.com/alexremn/kubectl-fieldlord/pkg/render"
)

// fakeInfos wraps unstructured objects as resource.Info, as kube.Resolve would.
func fakeInfos(us ...*unstructured.Unstructured) []*resource.Info {
	infos := make([]*resource.Info, 0, len(us))
	for _, u := range us {
		infos = append(infos, &resource.Info{Object: u, Name: u.GetName(), Namespace: u.GetNamespace()})
	}
	return infos
}

// fakeResolve returns a resolveFunc that ignores its args and yields the given objects.
func fakeResolve(us ...*unstructured.Unstructured) resolveFunc {
	return func(resource.RESTClientGetter, string, []string) ([]*resource.Info, error) {
		return fakeInfos(us...), nil
	}
}

func replicasMF(manager string) []metav1.ManagedFieldsEntry {
	return []metav1.ManagedFieldsEntry{{
		Manager: manager, Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "autoscaling/v2",
		FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)},
	}}
}

func deploy(name string, mf []metav1.ManagedFieldsEntry) *unstructured.Unstructured {
	u := &unstructured.Unstructured{}
	u.SetGroupVersionKind(schemaGVK("apps", "v1", "Deployment"))
	u.SetNamespace("default")
	u.SetName(name)
	u.SetManagedFields(mf)
	return u
}

func TestExplainEnvelope_JSON(t *testing.T) {
	u := deploy("api", []metav1.ManagedFieldsEntry{
		{Manager: "hpa", Operation: metav1.ManagedFieldsOperationUpdate, APIVersion: "autoscaling/v2",
			FieldsType: "FieldsV1", FieldsV1: &metav1.FieldsV1{Raw: []byte(`{"f:spec":{"f:replicas":{}}}`)}},
	})
	env := explainEnvelope(u, ownership.Build(u.GetManagedFields()), "v1.34.2", "full")
	var out bytes.Buffer
	if err := render.JSON(&out, env); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	for _, want := range []string{`"command": "explain"`, `".spec.replicas"`, `"hpa"`, `"schemaVersion": "v1"`, `"serverVersion": "v1.34.2"`} {
		if !strings.Contains(s, want) {
			t.Errorf("explain json missing %q:\n%s", want, s)
		}
	}
}

func TestRenderEnvelopes_SingleIsObject_MultiIsArray(t *testing.T) {
	e1 := explainEnvelope(deploy("a", nil), ownership.Model{}, "v1.34.2", "full")
	e2 := explainEnvelope(deploy("b", nil), ownership.Model{}, "v1.34.2", "full")

	var single bytes.Buffer
	if err := renderEnvelopes(&single, "json", []render.Envelope{e1}); err != nil {
		t.Fatal(err)
	}
	var obj render.Envelope
	if err := json.Unmarshal(single.Bytes(), &obj); err != nil {
		t.Fatalf("single output must be a JSON object: %v", err)
	}

	var multi bytes.Buffer
	if err := renderEnvelopes(&multi, "json", []render.Envelope{e1, e2}); err != nil {
		t.Fatal(err)
	}
	var arr []render.Envelope
	if err := json.Unmarshal(multi.Bytes(), &arr); err != nil {
		t.Fatalf("multi output must be a JSON array: %v", err)
	}
	if len(arr) != 2 {
		t.Errorf("want 2 envelopes, got %d", len(arr))
	}
}

func TestRunExplain_JSONMultiObjectIsArray(t *testing.T) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "json", skipVersionCheck: true},
		resolve: fakeResolve(deploy("a", replicasMF("hpa")), deploy("b", replicasMF("kubelet"))),
	}
	if err := runExplain(o, streams); err != nil {
		t.Fatalf("runExplain error = %v", err)
	}
	var arr []render.Envelope
	if err := json.Unmarshal(out.Bytes(), &arr); err != nil {
		t.Fatalf("multi-object json must be a single array: %v\n%s", err, out.String())
	}
	if len(arr) != 2 {
		t.Errorf("want 2 envelopes, got %d", len(arr))
	}
}

func TestRunExplain_TablePlain(t *testing.T) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "table", noColor: true, skipVersionCheck: true},
		resolve: fakeResolve(deploy("api", replicasMF("hpa"))),
	}
	if err := runExplain(o, streams); err != nil {
		t.Fatalf("runExplain error = %v", err)
	}
	s := out.String()
	for _, want := range []string{"Deployment/api", "FIELD", ".spec.replicas", "hpa"} {
		if !strings.Contains(s, want) {
			t.Errorf("table output missing %q:\n%s", want, s)
		}
	}
}

func TestRunExplain_ResolveError(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g: &globalOptions{output: "json", skipVersionCheck: true},
		resolve: func(resource.RESTClientGetter, string, []string) ([]*resource.Info, error) {
			return nil, fmt.Errorf("boom")
		},
	}
	if err := runExplain(o, streams); err == nil {
		t.Errorf("expected runExplain to propagate the resolve error")
	}
}
