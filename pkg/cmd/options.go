package cmd

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericclioptions"
)

// cmdOptions holds per-command state shared by explain and drift.
type cmdOptions struct {
	configFlags *genericclioptions.ConfigFlags
	g           *globalOptions
	namespace   string
	args        []string
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
