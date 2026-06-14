package cmd

import (
	"errors"
	"fmt"
	"io"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/alexremn/kubectl-fieldlord/pkg/drift"
	"github.com/alexremn/kubectl-fieldlord/pkg/kube"
	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
	"github.com/alexremn/kubectl-fieldlord/pkg/render"
)

func newDriftCmd(cf *genericclioptions.ConfigFlags, g *globalOptions, streams genericiooptions.IOStreams) *cobra.Command {
	o := &cmdOptions{configFlags: cf, g: g, resolve: kube.Resolve}
	var expectManager string
	cmd := &cobra.Command{
		Use:           "drift <resource>...",
		Short:         "Attribute ownership drift to a fieldManager",
		Example:       "  kubectl fieldlord drift deploy/api\n  kubectl fieldlord drift deploy/api --expect-manager helm",
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
			return runDrift(o, expectManager, streams)
		},
	}
	cmd.Flags().StringVar(&expectManager, "expect-manager", "", "Manager expected to own fields; others are drift (default: inferred primary applier)")
	return cmd
}

func runDrift(o *cmdOptions, expectManager string, streams genericiooptions.IOStreams) error {
	server, tier := probe(o, streams)
	infos, err := o.resolve(o.configFlags, o.namespace, o.args)
	if err != nil {
		return err
	}
	anyFindings := false
	for _, info := range infos {
		u, ok := info.Object.(*unstructured.Unstructured)
		if !ok {
			return fmt.Errorf("unexpected object type %T for %s", info.Object, info.Name)
		}
		rerr := renderDrift(streams.Out, o.g.output, o.g.noColor, u, expectManager, server, tier)
		if rerr != nil {
			var ee *ExitError
			if errors.As(rerr, &ee) && ee.Code == 2 {
				anyFindings = true
				continue
			}
			return rerr // real error (e.g. cannot infer primary) -> exit 1
		}
	}
	if anyFindings {
		return &ExitError{Code: 2}
	}
	return nil
}

// renderDrift renders one object and returns &ExitError{Code:2} when ATTRIBUTED
// drift exists. Unattributed findings are shown but do not by themselves gate.
func renderDrift(w io.Writer, output string, noColor bool, u *unstructured.Unstructured, expectManager, server, tier string) error {
	model := ownership.Build(u.GetManagedFields())
	findings, err := drift.Native(model, expectManager)
	if err != nil {
		return err // e.g. cannot infer primary -> exit 1
	}
	switch output {
	case "json", "yaml":
		env := driftEnvelope(u, findings, model.Warnings, server, tier)
		if output == "json" {
			err = render.JSON(w, env)
		} else {
			err = render.YAML(w, env)
		}
	default:
		fmt.Fprintf(w, "# %s/%s drift vs %s\n", u.GetKind(), u.GetName(), driftBaseline(expectManager))
		err = render.DriftTable(w, toRows(findings), noColor)
	}
	if err != nil {
		return err
	}
	if attributedCount(findings) > 0 {
		return &ExitError{Code: 2}
	}
	return nil
}

func attributedCount(findings []drift.Finding) int {
	n := 0
	for _, f := range findings {
		if f.Attributed {
			n++
		}
	}
	return n
}

func driftBaseline(expect string) string {
	if expect == "" {
		return "inferred primary applier"
	}
	return expect
}

func toRows(findings []drift.Finding) []render.DriftRow {
	rows := make([]render.DriftRow, 0, len(findings))
	for _, f := range findings {
		rows = append(rows, render.DriftRow{
			Path: f.Path, ExpectedManager: f.ExpectedManager,
			Attributed: f.Attributed, ActualOwner: f.ActualOwner,
		})
	}
	return rows
}

func driftEnvelope(u *unstructured.Unstructured, findings []drift.Finding, warnings []string, server, tier string) render.Envelope {
	gvk := u.GroupVersionKind()
	if warnings == nil {
		warnings = []string{}
	}
	return render.Envelope{
		SchemaVersion: render.SchemaVersion,
		Command:       "drift",
		Resource: render.ResourceRef{
			Group: gvk.Group, Version: gvk.Version, Kind: gvk.Kind,
			Namespace: u.GetNamespace(), Name: u.GetName(),
		},
		ServerVersion: server,
		SupportTier:   tier,
		Findings:      findings,
		Warnings:      warnings,
	}
}
