package ownership

import (
	"sort"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

// Build constructs the ownership Model from an object's managedFields entries.
// Each path lists every (manager,operation,apiVersion,subresource) owner that
// claims it. Output is deterministically sorted.
func Build(entries []metav1.ManagedFieldsEntry) Model {
	var m Model
	byPath := map[string][]Owner{}

	for _, e := range entries {
		owner, paths, warnings, err := decodeEntry(e)
		if err != nil {
			m.Warnings = append(m.Warnings, err.Error())
			continue
		}
		m.Warnings = append(m.Warnings, warnings...)
		for _, p := range paths {
			byPath[p] = appendOwnerUnique(byPath[p], owner)
		}
	}

	m.Paths = make([]OwnedPath, 0, len(byPath))
	for path, owners := range byPath {
		sortOwners(owners)
		m.Paths = append(m.Paths, OwnedPath{
			Path:       path,
			Atomic:     isAtomicPath(path),
			MultiOwner: countDistinctManagers(owners) > 1,
			Owners:     owners,
		})
	}
	sort.Slice(m.Paths, func(i, j int) bool { return m.Paths[i].Path < m.Paths[j].Path })
	return m
}

func appendOwnerUnique(owners []Owner, o Owner) []Owner {
	for _, ex := range owners {
		if ex == o {
			return owners
		}
	}
	return append(owners, o)
}

func sortOwners(owners []Owner) {
	sort.Slice(owners, func(i, j int) bool {
		a, b := owners[i], owners[j]
		switch {
		case a.Manager != b.Manager:
			return a.Manager < b.Manager
		case a.Operation != b.Operation:
			return a.Operation < b.Operation
		case a.APIVersion != b.APIVersion:
			return a.APIVersion < b.APIVersion
		default:
			return a.Subresource < b.Subresource
		}
	})
}

func countDistinctManagers(owners []Owner) int {
	seen := map[string]struct{}{}
	for _, o := range owners {
		seen[o.Manager] = struct{}{}
	}
	return len(seen)
}

// isAtomicPath reports whether the rendered path contains a positional list index
// element (e.g. "[0]"), which marks membership in an atomic list. Associative keys
// ("[name=...]") and set members ("[=...]") contain '=' and are not atomic. A
// richer schema-driven classification arrives in v0.2.
func isAtomicPath(path string) bool {
	open := strings.IndexByte(path, '[')
	for open >= 0 {
		closeIdx := strings.IndexByte(path[open:], ']')
		if closeIdx < 0 {
			return false
		}
		inner := path[open+1 : open+closeIdx]
		if inner != "" && isAllDigits(inner) {
			return true
		}
		rest := open + closeIdx + 1
		next := strings.IndexByte(path[rest:], '[')
		if next < 0 {
			return false
		}
		open = rest + next
	}
	return false
}

func isAllDigits(s string) bool {
	for i := 0; i < len(s); i++ {
		if s[i] < '0' || s[i] > '9' {
			return false
		}
	}
	return true
}
