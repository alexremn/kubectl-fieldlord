//go:build integration

package integration

import (
	"testing"

	"github.com/go-logr/logr"
	logf "sigs.k8s.io/controller-runtime/pkg/log"
)

// TestMain sets a logger before envtest starts: otherwise controller-runtime
// dumps a "log.SetLogger(...) was never called" stack trace into the test
// output once a run exceeds 30s (routine under -race).
func TestMain(m *testing.M) {
	logf.SetLogger(logr.Discard())
	m.Run()
}
