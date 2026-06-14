//go:build integration

package integration

import (
	"context"
	"encoding/json"
	"testing"
	"time"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/alexremn/kubectl-fieldlord/pkg/predict"
)

var (
	deployGVR = schema.GroupVersionResource{Group: "apps", Version: "v1", Resource: "deployments"}
	nsGVR     = schema.GroupVersionResource{Group: "", Version: "v1", Resource: "namespaces"}
)

// deploymentApplyConfig builds a minimal Deployment apply config as a plain map
// so it can be marshalled to JSON without importing typed apply-config packages.
func deploymentApplyConfig(name, namespace string, replicas int32) ([]byte, error) {
	obj := map[string]interface{}{
		"apiVersion": "apps/v1",
		"kind":       "Deployment",
		"metadata": map[string]interface{}{
			"name":      name,
			"namespace": namespace,
		},
		"spec": map[string]interface{}{
			"replicas": replicas,
			"selector": map[string]interface{}{
				"matchLabels": map[string]interface{}{"app": name},
			},
			"template": map[string]interface{}{
				"metadata": map[string]interface{}{
					"labels": map[string]interface{}{"app": name},
				},
				"spec": map[string]interface{}{
					"containers": []interface{}{
						map[string]interface{}{
							"name":  "c",
							"image": "nginx:stable",
						},
					},
				},
			},
		},
	}
	return json.Marshal(obj)
}

// namespaceApplyConfig builds a minimal Namespace apply config with one label.
func namespaceApplyConfig(name, labelVal string) ([]byte, error) {
	obj := map[string]interface{}{
		"apiVersion": "v1",
		"kind":       "Namespace",
		"metadata": map[string]interface{}{
			"name": name,
			"labels": map[string]interface{}{
				"env": labelVal,
			},
		},
	}
	return json.Marshal(obj)
}

// applySSA issues a server-side apply patch (no dry-run) using ApplyPatchType.
func applySSA(ctx context.Context, ri dynamic.ResourceInterface, name string, data []byte, manager string) error {
	_, err := ri.Patch(ctx, name, types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: manager,
	})
	return err
}

func TestEnvtest_Predict(t *testing.T) {
	env := &envtest.Environment{}
	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("start envtest (set KUBEBUILDER_ASSETS): %v", err)
	}
	defer func() { _ = env.Stop() }()

	dyn, err := dynamic.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), 60*time.Second)
	defer cancel()

	t.Run("namespaced_deployment_conflict", func(t *testing.T) {
		depRI := dyn.Resource(deployGVR).Namespace("default")

		// Persist ownership of spec.replicas=3 under mgr-a (no dry-run).
		setupData, err := deploymentApplyConfig("api", "default", 3)
		if err != nil {
			t.Fatalf("build setup apply config: %v", err)
		}
		if err := applySSA(ctx, depRI, "api", setupData, "mgr-a"); err != nil {
			t.Fatalf("setup apply as mgr-a: %v", err)
		}

		// Build a desired manifest mutating spec.replicas to 9 for mgr-b.
		desiredData, err := deploymentApplyConfig("api", "default", 9)
		if err != nil {
			t.Fatalf("build desired apply config: %v", err)
		}

		// Probe: Force=false, DryRun=All — should yield conflict on spec.replicas.
		conflicts, err := predict.Probe(ctx, depRI, "api", desiredData, "mgr-b")
		if err != nil {
			t.Fatalf("predict.Probe returned unexpected error: %v", err)
		}
		if len(conflicts) == 0 {
			t.Fatal("expected non-empty conflict set; got none")
		}

		var found bool
		for _, c := range conflicts {
			if c.Field == ".spec.replicas" && c.Manager == "mgr-a" {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("expected ConflictPath{Field:.spec.replicas, Manager:mgr-a}; got %+v", conflicts)
		}
	})

	t.Run("cluster_scoped_namespace_no_panic", func(t *testing.T) {
		nsRI := dyn.Resource(nsGVR) // no .Namespace(...) — cluster-scoped

		// Create the namespace under mgr-a.
		setupData, err := namespaceApplyConfig("scoped-demo", "staging")
		if err != nil {
			t.Fatalf("build namespace setup config: %v", err)
		}
		if err := applySSA(ctx, nsRI, "scoped-demo", setupData, "mgr-a"); err != nil {
			t.Fatalf("setup apply namespace as mgr-a: %v", err)
		}

		// Probe with mgr-b changing the label — exercises the cluster-scoped
		// ResourceInterface path; we only require it does not panic or return a
		// non-409 error (conflict set may be empty if the apiserver did not
		// record field ownership for simple label changes).
		desiredData, err := namespaceApplyConfig("scoped-demo", "production")
		if err != nil {
			t.Fatalf("build namespace desired config: %v", err)
		}
		_, err = predict.Probe(ctx, nsRI, "scoped-demo", desiredData, "mgr-b")
		if err != nil {
			t.Errorf("predict.Probe (cluster-scoped) returned unexpected error: %v", err)
		}
	})
}
