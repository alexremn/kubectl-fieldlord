package cmd

import (
	"context"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v6/typed"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
	"github.com/alexremn/kubectl-fieldlord/pkg/predict"
	"github.com/alexremn/kubectl-fieldlord/pkg/render"
)

// resolveFunc resolves kubectl-style resource refs to objects. It is a field on
// cmdOptions (defaulting to kube.Resolve) so commands can be unit-tested with a
// fake that returns canned objects instead of hitting a live cluster.
type resolveFunc func(getter resource.RESTClientGetter, namespace string, args []string) ([]*resource.Info, error)

// predictFunc runs the SSA dry-run probe for one object; injectable for tests.
type predictFunc func(ctx context.Context, info *resource.Info, data []byte, manager string) ([]predict.ConflictPath, error)

// diffFunc computes the typed desired-vs-live comparison; injectable for tests.
// The default wrapper (realDriftDiff) builds the TypeConverter internally, so this
// signature is converter-free.
type diffFunc func(desired, live *unstructured.Unstructured, managerOwned *fieldpath.Set) (*typed.Comparison, bool, error)

// cmdOptions holds per-command state shared by explain, drift, and predict.
type cmdOptions struct {
	configFlags *genericclioptions.ConfigFlags
	g           *globalOptions
	namespace   string
	args        []string
	resolve     resolveFunc
	predict     predictFunc
	diff        diffFunc
}

// predictFinding is the spec §9 predict envelope finding.
type predictFinding struct {
	Path          string           `json:"path"`
	LowConfidence bool             `json:"lowConfidence"`
	CurrentOwner  *ownership.Owner `json:"currentOwner,omitempty"`
}

func resourceRefOf(u *unstructured.Unstructured) render.ResourceRef {
	gvk := u.GroupVersionKind()
	return render.ResourceRef{
		Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind,
		Namespace: u.GetNamespace(), Name: u.GetName(),
	}
}

// ssaSupported reports whether a probed tier supports Server-Side Apply (>=1.22).
// An empty/"unknown" tier (probe skipped or failed) is treated as supported
// (do not block when we cannot determine the version).
func ssaSupported(tier string) bool {
	return tier == "" || tier == "unknown" || tier == "full"
}

// hasApplyManager reports whether manager already owns fields via an Apply op.
func hasApplyManager(entries []metav1.ManagedFieldsEntry, manager string) bool {
	for _, e := range entries {
		if e.Manager == manager && e.Operation == metav1.ManagedFieldsOperationApply {
			return true
		}
	}
	return false
}

// ownerForPath finds the owner of path in the model, preferring the named manager.
func ownerForPath(model ownership.Model, path, manager string) *ownership.Owner {
	for _, p := range model.Paths {
		if p.Path != path {
			continue
		}
		for _, o := range p.Owners {
			if o.Manager == manager {
				oo := o
				return &oo
			}
		}
		if len(p.Owners) > 0 {
			oo := p.Owners[0]
			return &oo
		}
	}
	return nil
}

func enrichConflicts(conflicts []predict.ConflictPath, model ownership.Model, lowConfidence bool) []predictFinding {
	findings := make([]predictFinding, 0, len(conflicts))
	for _, c := range conflicts {
		f := predictFinding{Path: c.Field, LowConfidence: lowConfidence}
		if o := ownerForPath(model, c.Field, c.Manager); o != nil {
			f.CurrentOwner = o
		} else if c.Manager != "" {
			f.CurrentOwner = &ownership.Owner{Manager: c.Manager}
		}
		findings = append(findings, f)
	}
	return findings
}

func validateOutput(o string) error {
	switch o {
	case "table", "json", "yaml":
		return nil
	default:
		return fmt.Errorf("unsupported output format %q (want table|json|yaml)", o)
	}
}

// resolveNamespace sets the effective namespace honoring -n and the context default.
func (o *cmdOptions) resolveNamespace() error {
	ns, _, err := o.configFlags.ToRawKubeConfigLoader().Namespace()
	if err != nil {
		return err
	}
	o.namespace = ns
	return nil
}
