package render

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

func osUnsetNoColor(t *testing.T) {
	t.Helper()
	old, had := os.LookupEnv("NO_COLOR")
	_ = os.Unsetenv("NO_COLOR")
	if had {
		t.Cleanup(func() { _ = os.Setenv("NO_COLOR", old) })
	}
}

func TestUseColor(t *testing.T) {
	t.Setenv("NO_COLOR", "") // present -> disables
	if useColor(false, true) {
		t.Errorf("NO_COLOR present must disable color")
	}
}

func TestUseColor_Matrix(t *testing.T) {
	osUnsetNoColor(t)
	if useColor(true, true) {
		t.Errorf("--no-color must disable")
	}
	if useColor(false, false) {
		t.Errorf("non-TTY must disable")
	}
	if !useColor(false, true) {
		t.Errorf("TTY + no flags + no NO_COLOR must enable")
	}
}

func TestExplainTable_NoColorPlain(t *testing.T) {
	model := ownership.Model{Paths: []ownership.OwnedPath{{
		Path: ".spec.replicas", MultiOwner: true,
		Owners: []ownership.Owner{
			{Manager: "hpa", Operation: ownership.OperationUpdate, APIVersion: "autoscaling/v2", Time: "2026-01-02T03:04:05Z"},
		},
	}}}
	var out bytes.Buffer
	if err := ExplainTable(&out, model, true /*noColor*/); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Contains(s, "\x1b[") {
		t.Errorf("no-color output must not contain ANSI escapes: %q", s)
	}
	for _, want := range []string{"FIELD", "MANAGER", "OPERATION", "APIVERSION", "SUBRESOURCE", "TIME", ".spec.replicas", "hpa", "Update", "2026-01-02T03:04:05Z"} {
		if !strings.Contains(s, want) {
			t.Errorf("table missing %q:\n%s", want, s)
		}
	}
}

func TestColorizeManager(t *testing.T) {
	if got := colorizeManager("helm", false); got != "helm" {
		t.Errorf("color off must return the plain name, got %q", got)
	}
	if got := colorizeManager("", true); got != "" {
		t.Errorf("empty name must stay empty, got %q", got)
	}
	colored := colorizeManager("helm", true)
	if !strings.HasPrefix(colored, "\x1b[") || !strings.Contains(colored, "helm") || !strings.HasSuffix(colored, "\x1b[0m") {
		t.Errorf("color on must wrap the name in ANSI escapes, got %q", colored)
	}
}

func TestDriftTable_NoColorPlain(t *testing.T) {
	rows := []DriftRow{
		{
			Path: ".spec.replicas", ExpectedManager: "helm", Attributed: true,
			ActualOwner: &ownership.Owner{Manager: "hpa", Operation: ownership.OperationUpdate},
		},
		{Path: ".spec.foo", ExpectedManager: "helm", Attributed: false, ActualOwner: nil},
	}
	var out bytes.Buffer
	if err := DriftTable(&out, rows, true); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Contains(s, "\x1b[") {
		t.Errorf("no-color drift table must not contain ANSI escapes: %q", s)
	}
	for _, want := range []string{"FIELD", "EXPECTED", "ACTUAL-MANAGER", "ATTRIBUTED", ".spec.replicas", "hpa", "Update", "true", "false"} {
		if !strings.Contains(s, want) {
			t.Errorf("drift table missing %q:\n%s", want, s)
		}
	}
}
