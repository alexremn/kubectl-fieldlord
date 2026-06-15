package drift

import (
	"testing"

	"github.com/alexremn/kubectl-fieldlord/pkg/ownership"
)

func mdl(paths ...ownership.OwnedPath) ownership.Model { return ownership.Model{Paths: paths} }
func mown(path, mgr string, op ownership.Operation) ownership.OwnedPath {
	return ownership.OwnedPath{Path: path, Owners: []ownership.Owner{{Manager: mgr, Operation: op}}}
}

func TestManifest_ConflictWhenModifiedOwnedByOther(t *testing.T) {
	m := mdl(mown(".spec.replicas", "hpa", ownership.OperationUpdate))
	f := Manifest(nil, []string{".spec.replicas"}, nil, m, "helm", true)
	if len(f) != 1 || f[0].Change != ChangeModified || !f[0].Conflict {
		t.Fatalf("modified field owned by hpa (not helm) must be a Conflict: %+v", f)
	}
	if f[0].ActualOwner == nil || f[0].ActualOwner.Manager != "hpa" {
		t.Errorf("actualOwner = %+v", f[0].ActualOwner)
	}
}

func TestManifest_SelfChangeNotConflict(t *testing.T) {
	m := mdl(mown(".spec.replicas", "helm", ownership.OperationApply))
	f := Manifest(nil, []string{".spec.replicas"}, nil, m, "helm", true)
	if f[0].Conflict {
		t.Errorf("change owned by expected manager is not a conflict: %+v", f[0])
	}
	if !f[0].Attributed {
		t.Errorf("should be attributed")
	}
}

func TestManifest_RemovalOwnedByOtherIsConflict(t *testing.T) {
	m := mdl(mown(".spec.foo", "argo", ownership.OperationApply))
	f := Manifest(nil, nil, []string{".spec.foo"}, m, "helm", true)
	if f[0].Change != ChangeRemoved || !f[0].Conflict {
		t.Errorf("removal owned by other must be a conflict: %+v", f[0])
	}
}

func TestManifest_AdditionInformational(t *testing.T) {
	f := Manifest([]string{".spec.newField"}, nil, nil, mdl(), "helm", true)
	if f[0].Change != ChangeAdded || f[0].Conflict || f[0].Attributed {
		t.Errorf("addition must be informational/unattributed/non-conflict: %+v", f[0])
	}
}

func TestManifest_EmptyExpectManagerNeverConflicts(t *testing.T) {
	m := mdl(mown(".spec.replicas", "hpa", ownership.OperationUpdate))
	f := Manifest(nil, []string{".spec.replicas"}, nil, m, "", true)
	if f[0].Conflict {
		t.Errorf("empty expectManager must never gate: %+v", f[0])
	}
	if !f[0].Attributed {
		t.Errorf("still attributed to hpa even when informational")
	}
}

func TestManifest_DegradedListUnattributed(t *testing.T) {
	m := mdl(mown(".spec.replicas", "helm", ownership.OperationApply))
	f := Manifest(nil, []string{".spec.containers"}, nil, m, "helm", false) // schemaUsed=false, path not in model
	if f[0].Granularity != "list" || f[0].Attributed || f[0].Conflict {
		t.Errorf("degraded list path must be unattributed/list/non-conflict: %+v", f[0])
	}
}

func TestManifest_SortedByPath(t *testing.T) {
	f := Manifest([]string{".b", ".a"}, []string{".c"}, nil, mdl(), "", true)
	if f[0].Path != ".a" || f[1].Path != ".b" || f[2].Path != ".c" {
		t.Errorf("must sort by path: %+v", f)
	}
}
