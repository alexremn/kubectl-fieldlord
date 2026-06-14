package cmd

import (
	"io"
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"
)

// executeArgs runs the full command tree with the given args, discarding output.
// It is used to exercise the cobra wiring and RunE validation guards that fail
// before any cluster access.
func executeArgs(t *testing.T, args ...string) error {
	t.Helper()
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	root := NewRootCmd(streams)
	root.SetArgs(args)
	root.SetOut(io.Discard)
	root.SetErr(io.Discard)
	return root.Execute()
}

func TestExplain_RejectsBadOutputFormat(t *testing.T) {
	if err := executeArgs(t, "explain", "deploy/x", "-o", "xml"); err == nil {
		t.Errorf("explain -o xml must return an error")
	}
}

func TestExplain_RequiresAResource(t *testing.T) {
	if err := executeArgs(t, "explain"); err == nil {
		t.Errorf("explain with no resource args must return an error")
	}
}

func TestDrift_RejectsBadOutputFormat(t *testing.T) {
	if err := executeArgs(t, "drift", "deploy/x", "-o", "xml"); err == nil {
		t.Errorf("drift -o xml must return an error")
	}
}

func TestDrift_RequiresAResource(t *testing.T) {
	if err := executeArgs(t, "drift"); err == nil {
		t.Errorf("drift with no resource args must return an error")
	}
}
