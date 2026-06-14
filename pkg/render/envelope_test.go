package render

import (
	"bytes"
	"encoding/json"
	"strings"
	"testing"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

func sampleEnvelope(name string) Envelope {
	return Envelope{
		SchemaVersion: SchemaVersion,
		Command:       "explain",
		Resource:      ResourceRef{Group: "apps", Version: "v1", Kind: "Deployment", Namespace: "default", Name: name},
		ServerVersion: "v1.34.2",
		SupportTier:   "full",
		Findings: []ownership.OwnedPath{{
			Path: ".spec.replicas", MultiOwner: true,
			Owners: []ownership.Owner{{Manager: "hpa", Operation: ownership.OperationUpdate, APIVersion: "autoscaling/v2"}},
		}},
		Warnings: []string{},
	}
}

func TestJSON_Deterministic(t *testing.T) {
	var a, b bytes.Buffer
	if err := JSON(&a, sampleEnvelope("api")); err != nil {
		t.Fatal(err)
	}
	if err := JSON(&b, sampleEnvelope("api")); err != nil {
		t.Fatal(err)
	}
	if a.String() != b.String() {
		t.Errorf("JSON not deterministic")
	}
	for _, want := range []string{`"schemaVersion": "v1"`, `"multiOwner": true`, `".spec.replicas"`} {
		if !strings.Contains(a.String(), want) {
			t.Errorf("JSON missing %q:\n%s", want, a.String())
		}
	}
}

func TestJSON_SliceIsArray(t *testing.T) {
	var out bytes.Buffer
	envs := []Envelope{sampleEnvelope("a"), sampleEnvelope("b")}
	if err := JSON(&out, envs); err != nil {
		t.Fatal(err)
	}
	var got []Envelope
	if err := json.Unmarshal(out.Bytes(), &got); err != nil {
		t.Fatalf("multi-envelope output must be a valid JSON array: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("want 2 envelopes, got %d", len(got))
	}
}

func TestYAML_RoundTrips(t *testing.T) {
	var out bytes.Buffer
	if err := YAML(&out, sampleEnvelope("api")); err != nil {
		t.Fatal(err)
	}
	if !strings.Contains(out.String(), "schemaVersion: v1") {
		t.Errorf("yaml missing schemaVersion: %s", out.String())
	}
}
