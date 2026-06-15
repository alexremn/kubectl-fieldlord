// Package drift derives ownership drift from a fieldlord ownership model.
package drift

import (
	"fmt"
	"sort"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

// Change is the kind of difference a manifest-mode finding represents.
type Change string

const (
	ChangeModified Change = "modified"
	ChangeAdded    Change = "added"
	ChangeRemoved  Change = "removed"
)

// Finding is one drifted field: owned by a manager other than the expected one.
type Finding struct {
	Path            string           `json:"path"`
	ExpectedManager string           `json:"expectedManager,omitempty"`
	Attributed      bool             `json:"attributed"`
	ActualOwner     *ownership.Owner `json:"actualOwner,omitempty"`
	Change          Change           `json:"change,omitempty"`
	Conflict        bool             `json:"conflict,omitempty"`
	Granularity     string           `json:"granularity,omitempty"`
}

// Native reports main-resource fields whose owner differs from the expected
// manager. Subresource-scoped fields (status/scale) are excluded from comparison
// against the main applier. If expectManager is empty, the primary applier is
// inferred: the Apply-operation manager owning the most leaf fields (tie-break:
// most leaves, then most-recent Time, then lexicographic name). Errors if no
// Apply manager exists and none was supplied.
func Native(m ownership.Model, expectManager string) ([]Finding, error) {
	expected := expectManager
	if expected == "" {
		inferred, ok := inferPrimary(m)
		if !ok {
			return nil, fmt.Errorf("cannot infer primary applier (no Apply-operation manager); pass --expect-manager")
		}
		expected = inferred
	}

	var findings []Finding
	for _, p := range m.Paths {
		mainOwners := mainResourceOwners(p)
		if len(mainOwners) == 0 {
			continue // subresource-only field: not compared against the main baseline
		}
		if ownersContain(mainOwners, expected) {
			continue
		}
		actual := mainOwners[0]
		findings = append(findings, Finding{
			Path:            p.Path,
			ExpectedManager: expected,
			Attributed:      true,
			ActualOwner:     &actual,
		})
	}
	sort.Slice(findings, func(i, j int) bool { return findings[i].Path < findings[j].Path })
	return findings, nil
}

func mainResourceOwners(p ownership.OwnedPath) []ownership.Owner {
	var out []ownership.Owner
	for _, o := range p.Owners {
		if o.Subresource == "" {
			out = append(out, o)
		}
	}
	return out
}

func ownersContain(owners []ownership.Owner, mgr string) bool {
	for _, o := range owners {
		if o.Manager == mgr {
			return true
		}
	}
	return false
}

func inferPrimary(m ownership.Model) (string, bool) {
	type stat struct {
		count  int
		latest string // max Owner.Time seen (RFC3339 lexicographic == chronological)
	}
	stats := map[string]*stat{}
	for _, p := range m.Paths {
		for _, o := range p.Owners {
			if o.Operation != ownership.OperationApply || o.Subresource != "" {
				continue
			}
			s := stats[o.Manager]
			if s == nil {
				s = &stat{}
				stats[o.Manager] = s
			}
			s.count++
			if o.Time > s.latest {
				s.latest = o.Time
			}
		}
	}
	if len(stats) == 0 {
		return "", false
	}
	best := ""
	for mgr, s := range stats {
		if best == "" {
			best = mgr
			continue
		}
		b := stats[best]
		switch {
		case s.count != b.count:
			if s.count > b.count {
				best = mgr
			}
		case s.latest != b.latest:
			if s.latest > b.latest {
				best = mgr
			}
		default:
			if mgr < best {
				best = mgr
			}
		}
	}
	return best, true
}
