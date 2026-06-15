package drift

import (
	"bytes"
	"fmt"
	"strings"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
	"k8s.io/apimachinery/pkg/apis/meta/v1/unstructured"
	"k8s.io/apimachinery/pkg/util/managedfields"
	"k8s.io/client-go/discovery"
	openapiclient "k8s.io/client-go/openapi"
	"k8s.io/client-go/openapi/cached"
	"sigs.k8s.io/structured-merge-diff/v6/fieldpath"
	"sigs.k8s.io/structured-merge-diff/v6/typed"
)

// BuildTypeConverter returns a schema-backed converter covering built-ins + any
// CRD publishing an OpenAPI v3 schema. Build ONCE per invocation and reuse.
func BuildTypeConverter(disco discovery.DiscoveryInterface) (managedfields.TypeConverter, error) {
	return openapiclient.NewTypeConverter(cached.NewClient(disco.OpenAPIV3()), false)
}

func getTypedValue(obj *unstructured.Unstructured, tc managedfields.TypeConverter) (*typed.TypedValue, bool, error) {
	tv, err := tc.ObjectToTyped(obj, typed.AllowDuplicates)
	if err == nil {
		return tv, true, nil
	}
	if isNoCorrespondingType(err) {
		dv, derr := typed.DeducedParseableType.FromUnstructured(obj.UnstructuredContent(), typed.AllowDuplicates)
		if derr != nil {
			return nil, false, derr
		}
		return dv, false, nil
	}
	return nil, false, err
}

// isNoCorrespondingType detects apimachinery's "no schema for this GVK" error. The
// concrete type + predicate are unexported, so substring match is the only stable
// detection — isolated here so a message change is a one-line fix + one test.
func isNoCorrespondingType(err error) bool {
	return err != nil && strings.Contains(err.Error(), "no corresponding type for")
}

// Diff frames Added/Modified/Removed as "what applying desired would do to live",
// then suppresses server-defaulting false-positives by intersecting ONLY Removed
// with managerOwned (Added/Modified are author-authored — intersecting Modified
// would hide real conflicts). schemaUsed is false if either side fell back to the
// deduced type. live=lhs, desired=rhs.
func Diff(desired, live *unstructured.Unstructured, tc managedfields.TypeConverter, managerOwned *fieldpath.Set) (*typed.Comparison, bool, error) {
	lt, ls, err := getTypedValue(live, tc)
	if err != nil {
		return nil, false, fmt.Errorf("typing live object: %w", err)
	}
	dt, ds, err := getTypedValue(desired, tc)
	if err != nil {
		return nil, false, fmt.Errorf("typing desired manifest: %w", err)
	}
	cmp, err := lt.Compare(dt)
	if err != nil {
		return nil, false, fmt.Errorf("comparing live vs desired: %w", err)
	}
	if !cmp.IsSame() {
		cmp.Removed = cmp.Removed.Intersection(managerOwned) // suppress defaulting noise
	}
	return cmp, ls && ds, nil
}

// ManagerOwnedSet is the union of fields manager owns on the MAIN resource (no
// subresources), reusing the decoder's nil-safe FieldsV1 access; tolerant skip-on-error.
func ManagerOwnedSet(entries []metav1.ManagedFieldsEntry, manager string) *fieldpath.Set {
	owned := &fieldpath.Set{}
	for _, mf := range entries {
		if mf.Manager != manager || mf.Subresource != "" || mf.FieldsV1 == nil {
			continue
		}
		raw := mf.FieldsV1.GetRawBytes()
		if len(raw) == 0 {
			continue
		}
		s := &fieldpath.Set{}
		if err := s.FromJSON(bytes.NewReader(raw)); err != nil {
			continue
		}
		owned = owned.Union(s)
	}
	return owned
}

// CollectChanged renders each changed leaf via fieldpath.Path.String() — identical
// to the ownership decoder, so the strings key into ownership.Model.
func CollectChanged(cmp *typed.Comparison) (added, modified, removed []string) {
	emit := func(s *fieldpath.Set) []string {
		var out []string
		s.Leaves().Iterate(func(p fieldpath.Path) { out = append(out, p.String()) })
		return out
	}
	return emit(cmp.Added), emit(cmp.Modified), emit(cmp.Removed)
}

// Scrub removes server-managed churn before diffing. The scrub list MUST contain
// ONLY server-managed fields (managerOwned is built from pre-scrub managedFields).
func Scrub(obj *unstructured.Unstructured, includeStatus bool) *unstructured.Unstructured {
	c := obj.DeepCopy()
	for _, p := range [][]string{
		{"metadata", "managedFields"}, {"metadata", "creationTimestamp"},
		{"metadata", "resourceVersion"}, {"metadata", "uid"},
		{"metadata", "generation"}, {"metadata", "selfLink"},
		{"metadata", "annotations", "kubectl.kubernetes.io/last-applied-configuration"},
	} {
		unstructured.RemoveNestedField(c.Object, p...)
	}
	if !includeStatus {
		unstructured.RemoveNestedField(c.Object, "status")
	}
	return c
}
