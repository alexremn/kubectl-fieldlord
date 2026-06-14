// Package predict runs a Server-Side Apply dry-run to predict which fields a
// forced apply would clobber, and which managers currently own them.
package predict

import (
	"context"
	"errors"
	"sort"
	"strconv"
	"strings"

	apierrors "k8s.io/apimachinery/pkg/api/errors"
	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/types"
	"k8s.io/client-go/dynamic"
	"k8s.io/utils/ptr"
)

// ConflictPath is one field a forced apply would clobber, with the owning manager
// scraped from the conflict message. Field is a structured-merge-diff path string
// identical to the ownership decoder's rendering (e.g. `.spec.replicas`).
type ConflictPath struct {
	Field   string
	Manager string
}

// Probe issues a non-force Server-Side Apply under DryRun:["All"] as fieldManager
// and returns the conflict set. No conflict -> (nil, nil). Non-409 errors (RBAC,
// network, webhook dry-run failure) propagate so the caller can report
// "could not predict" rather than "no conflicts".
func Probe(ctx context.Context, ri dynamic.ResourceInterface, name string, data []byte, fieldManager string) ([]ConflictPath, error) {
	_, err := ri.Patch(ctx, name, types.ApplyPatchType, data, metav1.PatchOptions{
		FieldManager: fieldManager,
		Force:        ptr.To(false),
		DryRun:       []string{metav1.DryRunAll},
	})
	return extractConflicts(err)
}

func extractConflicts(err error) ([]ConflictPath, error) {
	if err == nil {
		return nil, nil
	}
	if !apierrors.IsConflict(err) {
		return nil, err
	}
	var se *apierrors.StatusError
	if !errors.As(err, &se) {
		return nil, err
	}
	st := se.Status()
	if st.Details == nil {
		return nil, nil
	}
	var out []ConflictPath
	for _, c := range st.Details.Causes {
		if c.Type != metav1.CauseTypeFieldManagerConflict {
			continue
		}
		out = append(out, ConflictPath{Field: c.Field, Manager: parseManager(c.Message)})
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].Field != out[j].Field {
			return out[i].Field < out[j].Field
		}
		return out[i].Manager < out[j].Manager
	})
	return out, nil
}

// parseManager extracts the manager from `conflict with "<name>"...`, reading the
// first Go-quoted token (handles escaped quotes). Returns "" if the format differs.
func parseManager(msg string) string {
	rest, ok := strings.CutPrefix(msg, "conflict with ")
	if !ok {
		return ""
	}
	q, qerr := strconv.QuotedPrefix(rest)
	if qerr != nil {
		return ""
	}
	s, uerr := strconv.Unquote(q)
	if uerr != nil {
		return ""
	}
	return s
}
