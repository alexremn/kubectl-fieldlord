//go:build integration

package integration

import (
	"context"
	"testing"
	"time"

	appsv1 "k8s.io/api/apps/v1"
	corev1 "k8s.io/api/core/v1"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/client-go/kubernetes"
	"sigs.k8s.io/controller-runtime/pkg/envtest"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

func TestEnvtest_ManagedFieldsPresentAfterApply(t *testing.T) {
	env := &envtest.Environment{}
	cfg, err := env.Start()
	if err != nil {
		t.Fatalf("start envtest (set KUBEBUILDER_ASSETS): %v", err)
	}
	defer func() { _ = env.Stop() }()

	cs, err := kubernetes.NewForConfig(cfg)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()

	replicas := int32(2)
	dep := &appsv1.Deployment{
		ObjectMeta: metav1.ObjectMeta{Name: "api", Namespace: "default"},
		Spec: appsv1.DeploymentSpec{
			Replicas: &replicas,
			Selector: &metav1.LabelSelector{MatchLabels: map[string]string{"app": "api"}},
			Template: corev1.PodTemplateSpec{
				ObjectMeta: metav1.ObjectMeta{Labels: map[string]string{"app": "api"}},
				Spec:       corev1.PodSpec{Containers: []corev1.Container{{Name: "c", Image: "nginx"}}},
			},
		},
	}
	if _, err := cs.AppsV1().Deployments("default").Create(ctx, dep, metav1.CreateOptions{FieldManager: "itest"}); err != nil {
		t.Fatal(err)
	}
	got, err := cs.AppsV1().Deployments("default").Get(ctx, "api", metav1.GetOptions{})
	if err != nil {
		t.Fatal(err)
	}
	mf := got.GetManagedFields()
	if len(mf) == 0 {
		t.Fatal("expected managedFields on the fetched object")
	}
	model := ownership.Build(mf)
	var sawReplicas bool
	for _, p := range model.Paths {
		if p.Path == ".spec.replicas" {
			sawReplicas = true
		}
	}
	if !sawReplicas {
		t.Errorf("expected .spec.replicas in ownership model: %+v", model.Paths)
	}
}
