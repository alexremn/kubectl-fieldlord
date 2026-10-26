//go:build integration

package integration

import (
	"encoding/json"
	"errors"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/alexremn/kubectl-fieldlord/pkg/cmd"
	"github.com/alexremn/kubectl-fieldlord/pkg/drift"
	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

// TestCLI drives the real command tree end to end — kubeconfig loading,
// cli-runtime resolution, discovery, the OpenAPI v3 type converter, and the
// dynamic dry-run — i.e. the cluster glue unit tests replace with fakes.
func TestCLI(t *testing.T) {
	env := &envtest.Environment{}
	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("start envtest (set KUBEBUILDER_ASSETS): %v", err)
	}
	t.Cleanup(func() { _ = env.Stop() })

	user, err := env.AddUser(envtest.User{Name: "fieldlord", Groups: []string{"system:masters"}}, nil)
	if err != nil {
		t.Fatalf("add envtest user: %v", err)
	}
	kc, err := user.KubeConfig()
	if err != nil {
		t.Fatalf("user kubeconfig: %v", err)
	}
	dir := t.TempDir()
	kubeconfig := filepath.Join(dir, "kubeconfig")
	if err := os.WriteFile(kubeconfig, kc, 0o600); err != nil {
		t.Fatal(err)
	}

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		t.Fatalf("dynamic client: %v", err)
	}
	depRI := dyn.Resource(deployGVR).Namespace("default")

	// helm applies replicas=2, then "scaler" force-applies replicas=5 and takes
	// .spec.replicas away from helm: the drift/clobber every command reports.
	applyDeploymentSSA(t.Context(), t, depRI, deploymentManifest("cli-api", "default", "nginx:1", 2), "helm")
	scale, err := json.Marshal(map[string]interface{}{
		"apiVersion": "apps/v1", "kind": "Deployment",
		"metadata": map[string]interface{}{"name": "cli-api", "namespace": "default"},
		"spec":     map[string]interface{}{"replicas": 5},
	})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := depRI.Patch(t.Context(), "cli-api", types.ApplyPatchType, scale, metav1.PatchOptions{
		FieldManager: "scaler", Force: ptr.To(true),
	}); err != nil {
		t.Fatalf("force-apply as scaler: %v", err)
	}

	// helm's own manifest: replicas=2 now conflicts with scaler.
	desired := writeManifest(t, dir, "desired.json", deploymentManifest("cli-api", "default", "nginx:1", 2))

	run := func(args ...string) (stdout, stderr string, code int) {
		streams, _, out, errOut := genericiooptions.NewTestIOStreams()
		root := cmd.NewRootCmd(streams)
		root.SetArgs(append(args, "--kubeconfig", kubeconfig, "--cache-dir", filepath.Join(dir, "cache"), "-n", "default"))
		code = exitCode(root.Execute())
		return out.String(), errOut.String(), code
	}

	t.Run("explain_json", func(t *testing.T) {
		out, _, code := run("explain", "deploy/cli-api", "-o", "json")
		if code != 0 {
			t.Fatalf("exit %d, want 0\n%s", code, out)
		}
		var env struct {
			Command       string `json:"command"`
			ServerVersion string `json:"serverVersion"`
			SupportTier   string `json:"supportTier"`
			Resource      struct{ Kind, Name string }
			Findings      []ownership.OwnedPath `json:"findings"`
		}
		decode(t, out, &env)
		if env.Command != "explain" || env.Resource.Kind != "Deployment" || env.Resource.Name != "cli-api" {
			t.Errorf("envelope header = %q %s/%s", env.Command, env.Resource.Kind, env.Resource.Name)
		}
		if env.SupportTier != "full" || !strings.HasPrefix(env.ServerVersion, "v1.") {
			t.Errorf("probe = server %q tier %q, want v1.x/full", env.ServerVersion, env.SupportTier)
		}
		if got := ownerManagers(env.Findings, ".spec.replicas"); !slices.Equal(got, []string{"scaler"}) {
			t.Errorf(".spec.replicas owners = %v, want [scaler]", got)
		}
		if got := ownerManagers(env.Findings, `.spec.template.spec.containers[name="app"].image`); !slices.Equal(got, []string{"helm"}) {
			t.Errorf("image owners = %v, want [helm]", got)
		}
	})

	t.Run("drift_native_exit_two", func(t *testing.T) {
		out, _, code := run("drift", "deploy/cli-api", "--expect-manager", "helm", "-o", "json")
		if code != 2 {
			t.Fatalf("exit %d, want 2\n%s", code, out)
		}
		var env struct {
			Findings []drift.Finding `json:"findings"`
		}
		decode(t, out, &env)
		f := findDrift(env.Findings, ".spec.replicas")
		if f == nil || !f.Attributed || f.ActualOwner == nil || f.ActualOwner.Manager != "scaler" {
			t.Errorf(".spec.replicas finding = %+v, want attributed to scaler", f)
		}
	})

	t.Run("drift_manifest_conflict_exit_two", func(t *testing.T) {
		out, stderr, code := run("drift", "deploy/cli-api", "-f", desired, "--expect-manager", "helm", "-o", "json")
		if code != 2 {
			t.Fatalf("exit %d, want 2\nstdout: %s\nstderr: %s", code, out, stderr)
		}
		if strings.Contains(stderr, "no usable OpenAPI schema") {
			t.Errorf("built-in kind fell back to the deduced schema: %s", stderr)
		}
		var env struct {
			Findings []drift.Finding `json:"findings"`
		}
		decode(t, out, &env)
		f := findDrift(env.Findings, ".spec.replicas")
		if f == nil || f.Change != drift.ChangeModified || !f.Conflict || f.ActualOwner == nil || f.ActualOwner.Manager != "scaler" {
			t.Errorf(".spec.replicas finding = %+v, want modified conflict owned by scaler", f)
		}
	})

	t.Run("predict_conflict_exit_two", func(t *testing.T) {
		out, stderr, code := run("predict", "deploy/cli-api", "-f", desired, "--as-manager", "helm", "-o", "json")
		if code != 2 {
			t.Fatalf("exit %d, want 2\nstdout: %s\nstderr: %s", code, out, stderr)
		}
		var env struct {
			Findings []struct {
				Path         string           `json:"path"`
				CurrentOwner *ownership.Owner `json:"currentOwner"`
			} `json:"findings"`
		}
		decode(t, out, &env)
		if len(env.Findings) != 1 || env.Findings[0].Path != ".spec.replicas" ||
			env.Findings[0].CurrentOwner == nil || env.Findings[0].CurrentOwner.Manager != "scaler" {
			t.Errorf("findings = %+v, want only .spec.replicas owned by scaler", env.Findings)
		}
	})

	// The dry-run guarantee: a conflict-free predict must leave the object as is.
	t.Run("predict_clean_does_not_persist", func(t *testing.T) {
		before, err := depRI.Get(t.Context(), "cli-api", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		bump := writeManifest(t, dir, "bump.json", deploymentManifest("cli-api", "default", "nginx:2", 0))

		out, stderr, code := run("predict", "deploy/cli-api", "-f", bump, "--as-manager", "helm")
		if code != 0 {
			t.Fatalf("exit %d, want 0\nstdout: %s\nstderr: %s", code, out, stderr)
		}
		after, err := depRI.Get(t.Context(), "cli-api", metav1.GetOptions{})
		if err != nil {
			t.Fatal(err)
		}
		if after.GetResourceVersion() != before.GetResourceVersion() {
			t.Errorf("predict persisted a change: resourceVersion %s -> %s", before.GetResourceVersion(), after.GetResourceVersion())
		}
		if img := containerImage(t, after); img != "nginx:1" {
			t.Errorf("image = %q after predict, want unchanged nginx:1", img)
		}
	})

	t.Run("missing_resource_exit_one", func(t *testing.T) {
		_, _, code := run("explain", "deploy/does-not-exist")
		if code != 1 {
			t.Errorf("exit %d, want 1", code)
		}
	})
}

// exitCode mirrors cmd/kubectl-fieldlord's mapping of RunE errors to exit codes.
func exitCode(err error) int {
	if err == nil {
		return 0
	}
	var ee *cmd.ExitError
	if errors.As(err, &ee) {
		return ee.Code
	}
	return 1
}

func writeManifest(t *testing.T, dir, name string, obj map[string]interface{}) string {
	t.Helper()
	data, err := json.Marshal(obj)
	if err != nil {
		t.Fatal(err)
	}
	p := filepath.Join(dir, name)
	if err := os.WriteFile(p, data, 0o600); err != nil {
		t.Fatal(err)
	}
	return p
}

func decode(t *testing.T, out string, v any) {
	t.Helper()
	if err := json.Unmarshal([]byte(out), v); err != nil {
		t.Fatalf("invalid json output: %v\n%s", err, out)
	}
}

func ownerManagers(paths []ownership.OwnedPath, path string) []string {
	for _, p := range paths {
		if p.Path == path {
			out := make([]string, 0, len(p.Owners))
			for _, o := range p.Owners {
				out = append(out, o.Manager)
			}
			return out
		}
	}
	return nil
}

func findDrift(findings []drift.Finding, path string) *drift.Finding {
	for i := range findings {
		if findings[i].Path == path {
			return &findings[i]
		}
	}
	return nil
}

func containerImage(t *testing.T, u *unstructured.Unstructured) string {
	t.Helper()
	cs, _, err := unstructured.NestedSlice(u.Object, "spec", "template", "spec", "containers")
	if err != nil || len(cs) == 0 {
		t.Fatalf("containers: %v", err)
	}
	img, _, _ := unstructured.NestedString(cs[0].(map[string]interface{}), "image")
	return img
}
