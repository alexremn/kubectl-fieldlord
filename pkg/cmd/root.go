package cmd

import (
	"github.com/spf13/cobra"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/alexremn/kubectl-fieldlord/internal/buildinfo"
)

// globalOptions are flags shared by all subcommands (bound to root persistent flags).
type globalOptions struct {
	output           string
	noColor          bool
	skipVersionCheck bool
}

// NewRootCmd builds the `kubectl fieldlord` command tree. The returned command
// owns one shared *ConfigFlags and one *globalOptions exposed as persistent flags.
func NewRootCmd(streams genericiooptions.IOStreams) *cobra.Command {
	configFlags := genericclioptions.NewConfigFlags(true)
	g := &globalOptions{output: "table"}

	root := &cobra.Command{
		Use:           "kubectl-fieldlord",
		Short:         "Explain Server-Side Apply field ownership",
		Long:          "kubectl-fieldlord makes Kubernetes Server-Side Apply field ownership legible:\nexplain who owns each field and attribute ownership drift to a fieldManager.",
		Version:       buildinfo.Version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	pf := root.PersistentFlags()
	configFlags.AddFlags(pf)
	pf.StringVarP(&g.output, "output", "o", "table", "Output format: table|json|yaml")
	pf.BoolVar(&g.noColor, "no-color", false, "Disable colored table output")
	pf.BoolVar(&g.skipVersionCheck, "skip-version-check", false, "Skip the server-version capability probe")

	root.AddCommand(newVersionCmd())
	root.AddCommand(newExplainCmd(configFlags, g, streams))
	root.AddCommand(newDriftCmd(configFlags, g, streams))
	// cobra auto-adds `completion` via ExecuteC(); do NOT add it here.
	return root
}

// TEMP STUB — deleted in Task 11.
func newDriftCmd(_ *genericclioptions.ConfigFlags, _ *globalOptions, _ genericiooptions.IOStreams) *cobra.Command {
	return &cobra.Command{Use: "drift"}
}
