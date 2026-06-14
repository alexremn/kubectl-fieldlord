package render

import (
	"bytes"
	"os"
	"strings"
	"testing"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

func os_unsetNoColor(t *testing.T) {
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
	os_unsetNoColor(t)
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
