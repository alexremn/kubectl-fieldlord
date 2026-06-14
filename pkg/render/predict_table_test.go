package render

import (
	"bytes"
	"strings"
	"testing"
)

func TestPredictTable_NoColorPlain(t *testing.T) {
	rows := []PredictRow{
		{Field: ".spec.replicas", Manager: "hpa", Operation: "Update"},
		{Field: `.spec.template.spec.containers[name="app"].image`, Manager: "kubectl", Operation: "Apply"},
	}
	var out bytes.Buffer
	if err := PredictTable(&out, rows, true); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if strings.Contains(s, "\x1b[") {
		t.Errorf("no-color must not contain ANSI: %q", s)
	}
	for _, want := range []string{"FIELD", "CLOBBERS-MANAGER", "OPERATION", ".spec.replicas", "hpa", "Update"} {
		if !strings.Contains(s, want) {
			t.Errorf("predict table missing %q:\n%s", want, s)
		}
	}
}
