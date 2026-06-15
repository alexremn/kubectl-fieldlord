package drift

import (
	"sort"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

// Manifest classifies changed leaf paths from a desired-vs-live diff. expectManager
// "" => nothing is a Conflict (manifest mode never infers a baseline). schemaUsed=false
// marks paths absent from the model as degraded list-granularity (unattributed).
func Manifest(added, modified, removed []string, model ownership.Model, expectManager string, schemaUsed bool) []Finding {
	var findings []Finding
	add := func(paths []string, change Change) {
		for _, p := range paths {
			findings = append(findings, classify(p, change, model, expectManager, schemaUsed))
		}
	}
	add(added, ChangeAdded)
	add(modified, ChangeModified)
	add(removed, ChangeRemoved)
	sort.Slice(findings, func(i, j int) bool {
		if findings[i].Path != findings[j].Path {
			return findings[i].Path < findings[j].Path
		}
		return findings[i].Change < findings[j].Change
	})
	return findings
}

func classify(path string, change Change, model ownership.Model, expectManager string, schemaUsed bool) Finding {
	f := Finding{Path: path, Change: change, ExpectedManager: expectManager}
	owners := mainOwnersOfPath(model, path)
	if len(owners) == 0 {
		if !schemaUsed {
			f.Granularity = "list" // coarse path that cannot join managedFields
		}
		return f
	}
	f.Attributed = true
	// Prefer the expected manager when it (co-)owns the path: a field the expected
	// manager owns, even jointly with others, is a self-change — not a conflict.
	if expectManager != "" {
		if o := findOwner(owners, expectManager); o != nil {
			f.ActualOwner = o
			return f
		}
	}
	o := owners[0]
	f.ActualOwner = &o
	// Changed and owned only by manager(s) other than the named baseline.
	if change != ChangeAdded && expectManager != "" {
		f.Conflict = true
	}
	return f
}

// mainOwnersOfPath returns the main-resource (Subresource=="") owners of path.
func mainOwnersOfPath(model ownership.Model, path string) []ownership.Owner {
	for _, p := range model.Paths {
		if p.Path != path {
			continue
		}
		var out []ownership.Owner
		for _, o := range p.Owners {
			if o.Subresource == "" {
				out = append(out, o)
			}
		}
		return out
	}
	return nil
}

func findOwner(owners []ownership.Owner, manager string) *ownership.Owner {
	for _, o := range owners {
		if o.Manager == manager {
			oo := o
			return &oo
		}
	}
	return nil
}
