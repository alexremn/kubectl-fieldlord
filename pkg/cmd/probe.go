package cmd

import (
	"fmt"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/alexremn/kubectl-fieldlord/pkg/kube"
)

// probe runs the capability probe and returns (serverVersion, tier) for the
// output envelope. It never blocks: probe failures warn to stderr and proceed.
// When --skip-version-check is set it returns empty strings (omitted from output).
func probe(o *cmdOptions, streams genericiooptions.IOStreams) (server, tier string) {
	if o.g.skipVersionCheck {
		return "", ""
	}
	disco, err := o.configFlags.ToDiscoveryClient()
	if err != nil {
		fmt.Fprintln(streams.ErrOut, "warning: could not determine server version; proceeding")
		return "", "unknown"
	}
	t, derr := kube.DetectTier(disco)
	if derr != nil {
		fmt.Fprintln(streams.ErrOut, "warning: could not determine server version; proceeding")
		return "", "unknown"
	}
	if t.Name == "unsupported" {
		fmt.Fprintf(streams.ErrOut, "warning: server %s is below the supported floor (1.18); managedFields may be incomplete\n", t.GitVersion)
	}
	return t.GitVersion, t.Name
}
