package cmd

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/resource"
)

// resolveFunc resolves kubectl-style resource refs to objects. It is a field on
// cmdOptions (defaulting to kube.Resolve) so commands can be unit-tested with a
// fake that returns canned objects instead of hitting a live cluster.
type resolveFunc func(getter resource.RESTClientGetter, namespace string, args []string) ([]*resource.Info, error)

// cmdOptions holds per-command state shared by explain and drift.
type cmdOptions struct {
	configFlags *genericclioptions.ConfigFlags
	g           *globalOptions
	namespace   string
	args        []string
	resolve     resolveFunc
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
