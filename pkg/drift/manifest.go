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
	owner := mainOwnerOfPath(model, path)
	if owner == nil {
		if !schemaUsed {
			f.Granularity = "list" // coarse path that cannot join managedFields
		}
		return f
	}
	f.ActualOwner = owner
	f.Attributed = true
	if change != ChangeAdded && expectManager != "" && owner.Manager != expectManager {
		f.Conflict = true
	}
	return f
}

func mainOwnerOfPath(model ownership.Model, path string) *ownership.Owner {
	for _, p := range model.Paths {
		if p.Path != path {
			continue
		}
		for _, o := range p.Owners {
			if o.Subresource == "" {
				oo := o
				return &oo
			}
		}
	}
	return nil
}
