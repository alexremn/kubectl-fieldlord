package ownership

import (
	"encoding/json"
	"flag"
	"os"
	"path/filepath"
	"testing"

	metav1 "k8s.io/apimachinery/pkg/apis/meta/v1"
)

var update = flag.Bool("update", false, "update golden files")

func TestBuild_Golden(t *testing.T) {
	raw, err := os.ReadFile(filepath.Join("..", "..", "internal", "testdata", "fieldsv1", "deployment.json"))
	if err != nil {
		t.Fatal(err)
	}
	var entries []metav1.ManagedFieldsEntry
	if err := json.Unmarshal(raw, &entries); err != nil {
		t.Fatal(err)
	}
	got, err := json.MarshalIndent(Build(entries).Paths, "", "  ")
	if err != nil {
		t.Fatal(err)
	}
	goldenPath := filepath.Join("..", "..", "internal", "testdata", "fieldsv1", "deployment.golden.json")
	if *update {
		if err := os.WriteFile(goldenPath, got, 0o644); err != nil {
			t.Fatal(err)
		}
	}
	want, err := os.ReadFile(goldenPath)
	if err != nil {
		t.Fatalf("read golden (run with -update to create): %v", err)
	}
	if string(got) != string(want) {
		t.Errorf("golden mismatch; run `go test ./pkg/ownership -run Golden -update`\n got: %s", got)
	}
}
