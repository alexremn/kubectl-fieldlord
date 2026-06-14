package ownership

import (
	"bytes"
	"fmt"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
)

// decodeEntry parses one managedFields entry into its Owner and the set of leaf
// field paths it claims. Returns soft warnings (e.g. an entry that decoded to no
// paths) rather than failing — forward-compatibility over strictness.
func decodeEntry(e metav1.ManagedFieldsEntry) (Owner, []string, []string, error) {
	owner := Owner{
		Manager:     e.Manager,
		Operation:   Operation(e.Operation),
		APIVersion:  e.APIVersion,
		Subresource: e.Subresource,
	}
	if e.Time != nil {
		owner.Time = e.Time.UTC().Format("2006-01-02T15:04:05Z")
	}

	var warnings []string
	if e.FieldsType != "" && e.FieldsType != "FieldsV1" {
		warnings = append(warnings, fmt.Sprintf("unexpected fieldsType %q for manager %q", e.FieldsType, e.Manager))
		return owner, nil, warnings, nil
	}
	if e.FieldsV1 == nil || len(e.FieldsV1.Raw) == 0 {
		return owner, nil, warnings, nil
	}

	set := &fieldpath.Set{}
	if err := set.FromJSON(bytes.NewReader(e.FieldsV1.Raw)); err != nil {
		return owner, nil, warnings, fmt.Errorf("decoding FieldsV1 for manager %q: %w", e.Manager, err)
	}

	var paths []string
	set.Leaves().Iterate(func(p fieldpath.Path) {
		paths = append(paths, p.String()) // String() copies; safe to retain
	})

	if len(paths) == 0 {
		warnings = append(warnings, fmt.Sprintf("manager %q has a non-empty field set that decoded to no leaf paths (possibly unknown path-element types)", e.Manager))
	}
	return owner, paths, warnings, nil
}
