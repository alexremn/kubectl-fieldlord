package main

import (
	"strings"
	"testing"

	"k8s.io/cli-runtime/pkg/genericiooptions"
)

func TestRun_VersionExitsZero(t *testing.T) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	if code := run([]string{"version"}, streams); code != 0 {
		t.Errorf("version exit = %d, want 0", code)
	}
	if !strings.Contains(out.String(), "kubectl-fieldlord") {
		t.Errorf("version output missing program name: %q", out.String())
	}
}

func TestRun_MissingResourceExitsOne(t *testing.T) {
	streams, _, _, errOut := genericiooptions.NewTestIOStreams()
	// `explain` with no resource args errors before any cluster access -> exit 1.
	if code := run([]string{"explain"}, streams); code != 1 {
		t.Errorf("explain (no args) exit = %d, want 1", code)
	}
	if !strings.Contains(errOut.String(), "Error:") {
		t.Errorf("expected an error message on stderr, got %q", errOut.String())
	}
}

func TestRun_UnknownCommandExitsOne(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	if code := run([]string{"bogus-subcommand"}, streams); code != 1 {
		t.Errorf("unknown command exit = %d, want 1", code)
	}
}
