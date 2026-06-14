package cmd

import (
	"context"
	"fmt"
	"io"
	"os"

	"github.com/spf13/cobra"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/cli-runtime/pkg/genericclioptions"
	"k8s.io/cli-runtime/pkg/genericiooptions"
	"k8s.io/cli-runtime/pkg/resource"

	"github.com/alexremn/kubectl-fieldlord/pkg/kube"
	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
	"github.com/alexremn/kubectl-fieldlord/pkg/predict"
	"github.com/alexremn/kubectl-fieldlord/pkg/render"
)

const knownBuggyConflictMinor = 27 // kubernetes #119141, ~v1.27.x

func newPredictCmd(cf *genericclioptions.ConfigFlags, g *globalOptions, streams genericiooptions.IOStreams) *cobra.Command {
	o := &cmdOptions{configFlags: cf, g: g, resolve: kube.Resolve, predict: realPredict(cf)}
	var manifestPath, asManager string
	cmd := &cobra.Command{
		Use:           "predict <resource> -f <manifest> --as-manager <name>",
		Short:         "Predict which fields a --force-conflicts apply would clobber",
		Example:       "  kubectl fieldlord predict deploy/api -f desired.yaml --as-manager argocd-controller",
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(_ *cobra.Command, args []string) error {
			o.args = args
			if err := validateOutput(o.g.output); err != nil {
				return err
			}
			if len(args) != 1 {
				return fmt.Errorf("predict requires exactly one resource (e.g. deploy/api)")
			}
			if manifestPath == "" {
				return fmt.Errorf("predict requires -f <manifest> (use - for stdin)")
			}
			if asManager == "" {
				return fmt.Errorf("predict requires --as-manager <name>")
			}
			data, err := readManifest(manifestPath, streams.In)
			if err != nil {
				return err
			}
			if err := o.resolveNamespace(); err != nil {
				return err
			}
			return runPredict(o, data, asManager, streams)
		},
	}
	cmd.Flags().StringVarP(&manifestPath, "filename", "f", "", "Desired manifest (YAML or JSON; - for stdin)")
	cmd.Flags().StringVar(&asManager, "as-manager", "", "Field manager whose forced apply to simulate (required)")
	return cmd
}

func readManifest(path string, stdin io.Reader) ([]byte, error) {
	if path == "-" {
		return io.ReadAll(stdin)
	}
	return os.ReadFile(path) // G304: path is an operator-supplied -f flag, excluded in .golangci.yml
}

func runPredict(o *cmdOptions, data []byte, manager string, streams genericiooptions.IOStreams) error {
	server, tier, minor, probed := predictTier(o, streams)
	if probed && !ssaSupported(tier) {
		return fmt.Errorf("predict requires SSA (Kubernetes >= 1.22); server is %s", server)
	}

	infos, err := o.resolve(o.configFlags, o.namespace, o.args)
	if err != nil {
		return err
	}
	if len(infos) != 1 {
		return fmt.Errorf("predict requires exactly one resource, got %d", len(infos))
	}
	info := infos[0]
	u, ok := info.Object.(*unstructured.Unstructured)
	if !ok {
		return fmt.Errorf("unexpected object type %T for %s", info.Object, info.Name)
	}

	mf := u.GetManagedFields()
	if !hasApplyManager(mf, manager) {
		fmt.Fprintf(streams.ErrOut, "warning: %q is not an existing Apply manager on %s/%s; the prediction may be meaningless\n",
			manager, u.GetKind(), u.GetName())
	}

	conflicts, perr := o.predict(context.TODO(), info, data, manager)
	if perr != nil {
		return fmt.Errorf("could not predict for %s/%s: %w", u.GetKind(), u.GetName(), perr)
	}

	lowConf := minor == knownBuggyConflictMinor
	if lowConf && len(conflicts) > 0 {
		fmt.Fprintln(streams.ErrOut, "warning: server ~1.27 may report an inaccurate conflict set (kubernetes #119141)")
	}
	findings := enrichConflicts(conflicts, ownership.Build(mf), lowConf)

	if rerr := renderPredict(streams.Out, o.g.output, o.g.noColor, u, findings, server, tier); rerr != nil {
		return rerr
	}
	if len(findings) > 0 {
		return &ExitError{Code: 2}
	}
	return nil
}

// predictTier probes the server tier for the predict floor + #119141 gating.
func predictTier(o *cmdOptions, streams genericiooptions.IOStreams) (server, tier string, minor int, probed bool) {
	if o.g.skipVersionCheck {
		return "", "", 0, false
	}
	disco, err := o.configFlags.ToDiscoveryClient()
	if err != nil {
		fmt.Fprintln(streams.ErrOut, "warning: could not determine server version; proceeding")
		return "", "unknown", 0, false
	}
	t, derr := kube.DetectTier(disco)
	if derr != nil {
		fmt.Fprintln(streams.ErrOut, "warning: could not determine server version; proceeding")
		return "", "unknown", 0, false
	}
	return t.GitVersion, t.Name, t.Minor, true
}

func renderPredict(w io.Writer, output string, noColor bool, u *unstructured.Unstructured, findings []predictFinding, server, tier string) error {
	switch output {
	case "json", "yaml":
		env := render.Envelope{
			SchemaVersion: render.SchemaVersion,
			Command:       "predict",
			Resource:      resourceRefOf(u),
			ServerVersion: server,
			SupportTier:   tier,
			Findings:      findings,
			Warnings:      []string{},
		}
		if output == "json" {
			return render.JSON(w, env)
		}
		return render.YAML(w, env)
	default:
		fmt.Fprintf(w, "# %s/%s would-clobber set\n", u.GetKind(), u.GetName())
		return render.PredictTable(w, toPredictRows(findings), noColor)
	}
}

func toPredictRows(findings []predictFinding) []render.PredictRow {
	rows := make([]render.PredictRow, 0, len(findings))
	for _, f := range findings {
		r := render.PredictRow{Field: f.Path}
		if f.CurrentOwner != nil {
			r.Manager = f.CurrentOwner.Manager
			r.Operation = string(f.CurrentOwner.Operation)
		}
		rows = append(rows, r)
	}
	return rows
}

// realPredict builds the dynamic client and runs predict.Probe (the production path).
func realPredict(cf *genericclioptions.ConfigFlags) predictFunc {
	return func(ctx context.Context, info *resource.Info, data []byte, manager string) ([]predict.ConflictPath, error) {
		dyn, err := kube.DynamicClient(cf)
		if err != nil {
			return nil, err
		}
		ri, err := kube.ResourceInterfaceForInfo(dyn, info)
		if err != nil {
			return nil, err
		}
		return predict.Probe(ctx, ri, info.Name, data, manager)
	}
}
