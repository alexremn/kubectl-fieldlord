package cmd

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/runtime/schema"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/alexremn/kubectl-fieldlord/pkg/kube"
	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
	"github.com/alexremn/kubectl-fieldlord/pkg/render"
)

func schemaGVK(g, v, k string) schema.GroupVersionKind {
	return schema.GroupVersionKind{Group: g, Version: v, Kind: k}
}

func newExplainCmd(cf *genericclioptions.ConfigFlags, g *globalOptions, streams genericiooptions.IOStreams) *cobra.Command {
	o := &cmdOptions{configFlags: cf, g: g}
	return &cobra.Command{
		Use:           "explain <resource>...",
		Aliases:       []string{"own", "who"},
		Short:         "Show per-field ownership from managedFields",
		Example:       "  kubectl fieldlord explain deploy/api\n  kubectl fieldlord explain deploy/api -o json",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			o.args = args
			if err := validateOutput(o.g.output); err != nil {
				return err
			}
			if len(args) == 0 {
				return fmt.Errorf("at least one resource (e.g. deploy/api) is required")
			}
			if err := o.resolveNamespace(); err != nil {
				return err
			}
			return runExplain(o, streams)
		},
	}
}

func runExplain(o *cmdOptions, streams genericiooptions.IOStreams) error {
	server, tier := probe(o, streams)
	infos, err := kube.Resolve(o.configFlags, o.namespace, o.args)
	if err != nil {
		return err
	}
	var envs []render.Envelope
	for _, info := range infos {
		u, ok := info.Object.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected object type %T for %s", info.Object, info.Name)
		}
		model := ownership.Build(u.GetManagedFields())
		if o.g.output == "table" {
			if err := renderExplainTable(streams.Out, o.g.noColor, u, model, server, tier); err != nil {
				return err
			}
			continue
		}
		envs = append(envs, explainEnvelope(u, model, server, tier))
	}
	if o.g.output != "table" {
		return renderEnvelopes(streams.Out, o.g.output, envs)
	}
	return nil
}

// renderEnvelopes writes one object when len==1, else a top-level array.
func renderEnvelopes(w io.Writer, output string, envs []render.Envelope) error {
	var v any = envs
	if len(envs) == 1 {
		v = envs[0]
	}
	if output == "json" {
		return render.JSON(w, v)
	}
	return render.YAML(w, v)
}

func renderExplainTable(w io.Writer, noColor bool, u *unstructured.Unstructured, m ownership.Model, server, tier string) error {
	fmt.Fprintf(w, "# %s/%s (server %s, tier %s)\n", u.GetKind(), u.GetName(), server, tier)
	if err := render.ExplainTable(w, m, noColor); err != nil {
		return err
	}
	for _, warn := range m.Warnings {
		fmt.Fprintf(w, "warning: %s\n", warn)
	}
	return nil
}

func explainEnvelope(u *unstructured.Unstructured, m ownership.Model, server, tier string) render.Envelope {
	gvk := u.GroupVersionKind()
	warnings := m.Warnings
	if warnings == nil {
		warnings = []string{}
	}
	return render.Envelope{
		SchemaVersion: render.SchemaVersion,
		Command:       "explain",
		Resource: render.ResourceRef{
			Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind,
			Namespace: u.GetNamespace(), Name: u.GetName(),
		},
		ServerVersion: server,
		SupportTier:   tier,
		Findings:      m.Paths,
		Warnings:      warnings,
	}
}
