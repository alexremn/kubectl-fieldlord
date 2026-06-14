package render

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

// ExplainTable writes the per-field ownership table. Manager names are colorized
// when output is a terminal and color is not disabled.
func ExplainTable(w io.Writer, m ownership.Model, noColor bool) error {
	on := useColor(noColor, isTerminal(w))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "FIELD\tMANAGER\tOPERATION\tAPIVERSION\tSUBRESOURCE\tTIME\tMULTI"); err != nil {
		return err
	}
	for _, p := range m.Paths {
		multi := ""
		if p.MultiOwner {
			multi = "*"
		}
		for i, o := range p.Owners {
			field := p.Path
			if i > 0 {
				field = "" // collapse repeated path for readability
			}
			if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\t%s\n",
				field, colorizeManager(o.Manager, on), o.Operation, o.APIVersion, o.Subresource, o.Time, multi); err != nil {
				return err
			}
		}
	}
	return tw.Flush()
}

// DriftRow is the rendering shape for a drift finding (mirrors drift.Finding,
// duplicated here so render does not import drift).
type DriftRow struct {
	Path            string
	ExpectedManager string
	Attributed      bool
	ActualOwner     *ownership.Owner
}

// DriftTable writes drift findings as an aligned table.
func DriftTable(w io.Writer, findings []DriftRow, noColor bool) error {
	on := useColor(noColor, isTerminal(w))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "FIELD\tEXPECTED\tACTUAL-MANAGER\tOPERATION\tATTRIBUTED"); err != nil {
		return err
	}
	for _, f := range findings {
		mgr, op := "", ""
		if f.ActualOwner != nil {
			mgr, op = colorizeManager(f.ActualOwner.Manager, on), string(f.ActualOwner.Operation)
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%t\n",
			f.Path, colorizeManager(f.ExpectedManager, on), mgr, op, f.Attributed); err != nil {
			return err
		}
	}
	return tw.Flush()
}
