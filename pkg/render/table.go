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
	Change          string // manifest diff summary; blank for native-mode rows
	Attributed      bool
	ActualOwner     *ownership.Owner
}

// DriftTable writes drift findings as an aligned table.
func DriftTable(w io.Writer, findings []DriftRow, noColor bool) error {
	on := useColor(noColor, isTerminal(w))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "FIELD\tEXPECTED\tACTUAL-MANAGER\tOPERATION\tCHANGE\tATTRIBUTED"); err != nil {
		return err
	}
	for _, f := range findings {
		mgr, op := "", ""
		if f.ActualOwner != nil {
			mgr, op = colorizeManager(f.ActualOwner.Manager, on), string(f.ActualOwner.Operation)
		}
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%t\n",
			f.Path, colorizeManager(f.ExpectedManager, on), mgr, op, f.Change, f.Attributed); err != nil {
			return err
		}
	}
	return tw.Flush()
}

// PredictRow is one predicted clobber for table rendering.
type PredictRow struct {
	Field     string
	Manager   string
	Operation string
}

// PredictTable writes the predicted clobber set: which fields a forced apply would
// overwrite and which manager+operation currently owns each.
func PredictTable(w io.Writer, rows []PredictRow, noColor bool) error {
	on := useColor(noColor, isTerminal(w))
	tw := tabwriter.NewWriter(w, 0, 4, 2, ' ', 0)
	if _, err := fmt.Fprintln(tw, "FIELD\tCLOBBERS-MANAGER\tOPERATION"); err != nil {
		return err
	}
	for _, r := range rows {
		if _, err := fmt.Fprintf(tw, "%s\t%s\t%s\n", r.Field, colorizeManager(r.Manager, on), r.Operation); err != nil {
			return err
		}
	}
	return tw.Flush()
}
