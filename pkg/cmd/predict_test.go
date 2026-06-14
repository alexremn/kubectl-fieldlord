package cmd

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"os"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
	"github.com/alexremn/kubectl-fieldlord/pkg/predict"
)

func fakePredict(c []predict.ConflictPath, err error) predictFunc {
	return func(context.Context, *resource.Info, []byte, string) ([]predict.ConflictPath, error) {
		return c, err
	}
}

func TestRunPredict_ExitsTwoWithCurrentOwner(t *testing.T) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "json", skipVersionCheck: true},
		resolve: fakeResolve(deploy("api", replicasMF("hpa"))),
		predict: fakePredict([]predict.ConflictPath{{Field: ".spec.replicas", Manager: "hpa"}}, nil),
	}
	err := runPredict(o, []byte("{}"), "helm", streams)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("want ExitError 2, got %v", err)
	}
	var env map[string]any
	if jerr := json.Unmarshal(out.Bytes(), &env); jerr != nil {
		t.Fatalf("invalid json: %v\n%s", jerr, out.String())
	}
	if env["command"] != "predict" {
		t.Errorf("command=%v", env["command"])
	}
	findings := env["findings"].([]any)
	f0 := findings[0].(map[string]any)
	if f0["path"] != ".spec.replicas" {
		t.Errorf("finding path=%v", f0["path"])
	}
	if _, ok := f0["currentOwner"]; !ok {
		t.Errorf("finding missing currentOwner: %v", f0)
	}
	if _, ok := f0["lowConfidence"]; !ok {
		t.Errorf("finding missing lowConfidence: %v", f0)
	}
}

func TestRunPredict_CleanIsZero(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "table", noColor: true, skipVersionCheck: true},
		resolve: fakeResolve(deploy("api", nil)),
		predict: fakePredict(nil, nil),
	}
	if err := runPredict(o, []byte("{}"), "helm", streams); err != nil {
		t.Fatalf("clean must not error: %v", err)
	}
}

func TestRunPredict_CouldNotPredictIsExitOne(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "table", noColor: true, skipVersionCheck: true},
		resolve: fakeResolve(deploy("api", nil)),
		predict: fakePredict(nil, errors.New("webhook dry-run failed")),
	}
	err := runPredict(o, []byte("{}"), "helm", streams)
	var ee *ExitError
	if errors.As(err, &ee) && ee.Code == 2 {
		t.Fatalf("could-not-predict must NOT be exit 2")
	}
	if err == nil {
		t.Fatalf("could-not-predict must be a non-nil error (exit 1)")
	}
}

func TestRunPredict_AsManagerNoMatchWarns(t *testing.T) {
	streams, _, _, errOut := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "table", noColor: true, skipVersionCheck: true},
		resolve: fakeResolve(deploy("api", replicasMF("hpa"))), // hpa is Update, no Apply mgr
		predict: fakePredict(nil, nil),
	}
	_ = runPredict(o, []byte("{}"), "ghost", streams)
	if !strings.Contains(errOut.String(), "ghost") {
		t.Errorf("expected no-match warning mentioning the manager; got %q", errOut.String())
	}
}

func TestRunPredict_RequiresSingleResource(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "table", skipVersionCheck: true},
		resolve: fakeResolve(deploy("a", nil), deploy("b", nil)),
		predict: fakePredict(nil, nil),
	}
	if err := runPredict(o, []byte("{}"), "helm", streams); err == nil || !strings.Contains(err.Error(), "exactly one") {
		t.Errorf("multiple resources must error 'exactly one'; got %v", err)
	}
}

func TestRunPredict_ResolveError(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g: &globalOptions{output: "table", skipVersionCheck: true},
		resolve: func(resource.RESTClientGetter, string, []string) ([]*resource.Info, error) {
			return nil, errors.New("boom")
		},
		predict: fakePredict(nil, nil),
	}
	if err := runPredict(o, []byte("{}"), "helm", streams); err == nil {
		t.Errorf("resolve error must propagate")
	}
}

func TestRunPredict_YAMLEnvelope(t *testing.T) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "yaml", skipVersionCheck: true},
		resolve: fakeResolve(deploy("api", replicasMF("hpa"))),
		predict: fakePredict([]predict.ConflictPath{{Field: ".spec.replicas", Manager: "hpa"}}, nil),
	}
	_ = runPredict(o, []byte("{}"), "helm", streams)
	if !strings.Contains(out.String(), "command: predict") {
		t.Errorf("yaml missing command: predict:\n%s", out.String())
	}
}

func TestReadManifest(t *testing.T) {
	got, err := readManifest("-", strings.NewReader("hello"))
	if err != nil || string(got) != "hello" {
		t.Errorf("stdin read: %q %v", got, err)
	}
	if _, err := readManifest("/nonexistent/x.yaml", nil); err == nil {
		t.Errorf("missing file must error")
	}
}

// TestExitError_ErrorAndUnwrap exercises the zero-Err branch (Code-only exit)
// and the Err-carrying branch so exit.go hits 100%.
func TestExitError_ErrorAndUnwrap(t *testing.T) {
	// Code-only — Error() must return ""
	e1 := &ExitError{Code: 2}
	if got := e1.Error(); got != "" {
		t.Errorf("Code-only ExitError.Error() = %q, want \"\"", got)
	}
	if e1.Unwrap() != nil {
		t.Errorf("Code-only Unwrap() must be nil")
	}
	// Err-carrying — Error() must return the inner message
	inner := errors.New("inner")
	e2 := &ExitError{Code: 1, Err: inner}
	if got := e2.Error(); got != "inner" {
		t.Errorf("Err-carrying ExitError.Error() = %q, want %q", got, "inner")
	}
	if !errors.Is(e2, inner) {
		t.Errorf("Unwrap() must expose inner error via errors.Is")
	}
}

// TestToPredictRows exercises the nil-CurrentOwner branch (no owner on finding).
func TestToPredictRows(t *testing.T) {
	findings := []predictFinding{
		{Path: ".spec.replicas", CurrentOwner: &ownership.Owner{Manager: "hpa", Operation: ownership.OperationUpdate}},
		{Path: ".spec.paused"}, // no currentOwner — empty manager/op
	}
	rows := toPredictRows(findings)
	if len(rows) != 2 {
		t.Fatalf("want 2 rows, got %d", len(rows))
	}
	if rows[0].Manager != "hpa" || rows[0].Operation != "Update" {
		t.Errorf("rows[0] = %+v", rows[0])
	}
	if rows[1].Manager != "" || rows[1].Operation != "" {
		t.Errorf("rows[1] should have empty manager/op: %+v", rows[1])
	}
}

// TestOwnerForPath_FallbackFirstOwner covers the branch where the named manager
// is absent from the path's owner list but other owners exist.
func TestOwnerForPath_FallbackFirstOwner(t *testing.T) {
	model := ownership.Model{Paths: []ownership.OwnedPath{{
		Path: ".spec.replicas",
		Owners: []ownership.Owner{
			{Manager: "hpa", Operation: ownership.OperationUpdate},
			{Manager: "kubectl", Operation: ownership.OperationApply},
		},
	}}}
	// Ask for "argocd" which isn't an owner — should fall back to first owner (hpa).
	got := ownerForPath(model, ".spec.replicas", "argocd")
	if got == nil || got.Manager != "hpa" {
		t.Errorf("expected fallback to first owner hpa, got %+v", got)
	}
	// Path not in model at all — should return nil.
	if got2 := ownerForPath(model, ".spec.unknown", "hpa"); got2 != nil {
		t.Errorf("absent path must return nil, got %+v", got2)
	}
}

// TestRunPredict_TableWithFindings exercises renderPredict's table branch with
// actual findings, which drives toPredictRows through a non-empty path.
func TestRunPredict_TableWithFindings(t *testing.T) {
	streams, _, out, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{
		g:       &globalOptions{output: "table", noColor: true, skipVersionCheck: true},
		resolve: fakeResolve(deploy("api", replicasMF("hpa"))),
		predict: fakePredict([]predict.ConflictPath{{Field: ".spec.replicas", Manager: "hpa"}}, nil),
	}
	err := runPredict(o, []byte("{}"), "helm", streams)
	var ee *ExitError
	if !errors.As(err, &ee) || ee.Code != 2 {
		t.Fatalf("want ExitError 2, got %v", err)
	}
	s := out.String()
	if !strings.Contains(s, ".spec.replicas") {
		t.Errorf("table output missing .spec.replicas:\n%s", s)
	}
}

// TestPredictTier_SkipVersionCheck verifies the fast-path (no probe, probed=false).
func TestPredictTier_SkipVersionCheck(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	o := &cmdOptions{g: &globalOptions{skipVersionCheck: true}}
	server, tier, minor, probed := predictTier(o, streams)
	if probed || server != "" || tier != "" || minor != 0 {
		t.Errorf("skipVersionCheck must return empty/false: server=%q tier=%q minor=%d probed=%v", server, tier, minor, probed)
	}
}

// TestPredictTier_BadConfigFlagsWarns verifies that a bad kubeconfig produces a
// warning on stderr and returns (unknown, probed=false) rather than blocking.
func TestPredictTier_BadConfigFlagsWarns(t *testing.T) {
	streams, _, _, errOut := genericiooptions.NewTestIOStreams()
	bad := "/nonexistent/kubeconfig"
	cf := genericclioptions.NewConfigFlags(false)
	cf.KubeConfig = &bad
	o := &cmdOptions{
		g:           &globalOptions{skipVersionCheck: false},
		configFlags: cf,
	}
	_, tier, _, probed := predictTier(o, streams)
	if probed {
		t.Error("bad kubeconfig must return probed=false")
	}
	if tier != "unknown" {
		t.Errorf("bad kubeconfig must return tier=unknown, got %q", tier)
	}
	if !strings.Contains(errOut.String(), "warning") {
		t.Errorf("expected warning on stderr, got %q", errOut.String())
	}
}

// TestNewPredictCmd_MissingFlags exercises the RunE validation paths:
// missing -f and missing --as-manager.
func TestNewPredictCmd_MissingFlags(t *testing.T) {
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	cf := genericclioptions.NewConfigFlags(false)
	g := &globalOptions{output: "table", skipVersionCheck: true}
	cmd := newPredictCmd(cf, g, streams)
	// No args → not our check (cobra handles arg count for RunE), but no flags either.
	cmd.SetArgs([]string{"deploy/api"}) // has resource but no -f or --as-manager
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "-f") {
		t.Errorf("missing -f must error; got %v", err)
	}
}

// TestNewPredictCmd_MissingAsManager verifies the --as-manager validation branch.
func TestNewPredictCmd_MissingAsManager(t *testing.T) {
	// Write a temp file to satisfy -f validation
	tmp := t.TempDir() + "/m.yaml"
	if werr := os.WriteFile(tmp, []byte("{}"), 0o600); werr != nil {
		t.Fatalf("tmp file: %v", werr)
	}
	streams, _, _, _ := genericiooptions.NewTestIOStreams()
	cf := genericclioptions.NewConfigFlags(false)
	g := &globalOptions{output: "table", skipVersionCheck: true}
	cmd := newPredictCmd(cf, g, streams)
	cmd.SetArgs([]string{"deploy/api", "-f", tmp})
	err := cmd.Execute()
	if err == nil || !strings.Contains(err.Error(), "--as-manager") {
		t.Errorf("missing --as-manager must error; got %v", err)
	}
}

var _ = metav1.ManagedFieldsOperationApply
var _ = bytes.NewBuffer
