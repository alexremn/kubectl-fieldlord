package cmd

import (
	"bytes"
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/alexremn/kubectl-fieldlord/internal/buildinfo"
)

func TestRootCmd_HasSubcommands(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	root := NewRootCmd(streams)
	for _, name := range []string{"explain", "drift", "version"} {
		c, _, err := root.Find([]string{name})
		if err != nil || c.Name() != name {
			t.Errorf("subcommand %q not registered (err=%v)", name, err)
		}
	}
}

func TestRootCmd_GlobalFlags(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	root := NewRootCmd(streams)
	for _, f := range []string{"namespace", "output", "no-color", "skip-version-check"} {
		if root.PersistentFlags().Lookup(f) == nil {
			t.Errorf("expected persistent flag --%s", f)
		}
	}
}

func TestRootCmd_VersionFlag(t *testing.T) {
	old := buildinfo.Version
	t.Cleanup(func() { buildinfo.Version = old })
	buildinfo.Version = "9.9.9"
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	root := NewRootCmd(streams)
	var out bytes.Buffer
	root.SetOut(&out)
	root.SetArgs([]string{"--version"})
	if err := root.Execute(); err != nil {
		t.Fatalf("--version error = %v", err)
	}
	if !strings.Contains(out.String(), "9.9.9") {
		t.Errorf("--version output missing version: %q", out.String())
	}
}
