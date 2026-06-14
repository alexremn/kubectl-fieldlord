package kube

import (
	"testing"

	"k8s.io/apimachinery/pkg/version"
	fakediscovery "k8s.io/client-go/discovery/fake"
	clienttesting "k8s.io/client-go/testing"
)

func disco(info *version.Info) *fakediscovery.FakeDiscovery {
	return &fakediscovery.FakeDiscovery{Fake: &clienttesting.Fake{}, FakedServerVersion: info}
}

func TestDetectTier(t *testing.T) {
	tests := []struct {
		name        string
		info        *version.Info
		wantTier    string
		wantGitPart string
	}{
		{"full", &version.Info{Major: "1", Minor: "34", GitVersion: "v1.34.2"}, "full", "v1.34.2"},
		{"best-effort", &version.Info{Major: "1", Minor: "20", GitVersion: "v1.20.0"}, "best-effort", "v1.20.0"},
		{"unsupported", &version.Info{Major: "1", Minor: "16", GitVersion: "v1.16.0"}, "unsupported", "v1.16.0"},
		{"gke suffix", &version.Info{Major: "1", Minor: "29+", GitVersion: "v1.29.4-gke.1"}, "full", "v1.29.4-gke.1"},
		{"git fallback", &version.Info{GitVersion: "v1.31.4-gke.1"}, "full", "v1.31.4-gke.1"},
		{"no gitversion", &version.Info{Major: "1", Minor: "33"}, "full", "v1.33"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			tier, err := DetectTier(disco(tt.info))
			if err != nil {
				t.Fatalf("DetectTier error = %v", err)
			}
			if tier.Name != tt.wantTier {
				t.Errorf("tier = %q, want %q", tier.Name, tt.wantTier)
			}
			if tier.GitVersion != tt.wantGitPart {
				t.Errorf("gitVersion = %q, want %q", tier.GitVersion, tt.wantGitPart)
			}
		})
	}
}
