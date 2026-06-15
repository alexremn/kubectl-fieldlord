package render

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

func TestDriftTable_ChangeColumn(t *testing.T) {
	rows := []DriftRow{
		{Path: ".spec.replicas", ExpectedManager: "helm", Attributed: true, Change: "modified",
			ActualOwner: &ownership.Owner{Manager: "hpa", Operation: ownership.OperationUpdate}},
		{Path: ".spec.legacy", ExpectedManager: "helm", Attributed: true, Change: "", // native-mode row: blank CHANGE
			ActualOwner: &ownership.Owner{Manager: "hpa", Operation: ownership.OperationUpdate}},
	}
	var out bytes.Buffer
	if err := DriftTable(&out, rows, true); err != nil {
		t.Fatal(err)
	}
	s := out.String()
	if !strings.Contains(s, "CHANGE") {
		t.Errorf("header must include CHANGE:\n%s", s)
	}
	if !strings.Contains(s, "modified") {
		t.Errorf("manifest row must show change value:\n%s", s)
	}
}
