package cmd

import (
	"bytes"
	"strings"
	"testing"

	"github.com/alexremn/kubectl-fieldlord/internal/buildinfo"
)

func TestVersionCmd_PrintsVersion(t *testing.T) {
	old := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = old })
	buildinfo.Version = "1.2.3"

	cmd := newVersionCmd()
	var out bytes.Buffer
	cmd.SetOut(&out)
	if err := cmd.Execute(); err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if !strings.Contains(out.String(), "kubectl-fieldlord 1.2.3") {
		t.Errorf("version output missing version: %q", out.String())
	}
}
