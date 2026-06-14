// Package kube holds the thin Kubernetes client layer: resource resolution and
// the server-version capability probe.
package kube

import (
	"fmt"
	"regexp"
	"strconv"
	"strings"

	"k8s.io/client-go/discovery"
)

// Tier classifies the connected server's support level for fieldlord.
type Tier struct {
	Name       string // "full" (>=1.22), "best-effort" (1.18-1.21), "unsupported" (<1.18)
	Major      int
	Minor      int
	GitVersion string // e.g. "v1.34.2"; falls back to "v<major>.<minor>" if absent
}

var gitVersionRe = regexp.MustCompile(`^v?(\d+)\.(\d+)`)

// DetectTier probes the server version via discovery and classifies it. The
// caller decides what to do with an error (warn + proceed for explain/drift).
func DetectTier(disco discovery.DiscoveryInterface) (Tier, error) {
	info, err := disco.ServerVersion()
	if err != nil {
		return Tier{Name: "unknown"}, fmt.Errorf("querying server version: %w", err)
	}
	major, eMaj := parseVersionComponent(info.Major)
	minor, eMin := parseVersionComponent(info.Minor)
	if eMaj != nil || eMin != nil {
		m := gitVersionRe.FindStringSubmatch(info.GitVersion)
		if m == nil {
			return Tier{Name: "unknown"}, fmt.Errorf(
				"cannot parse server version: major=%q minor=%q gitVersion=%q",
				info.Major, info.Minor, info.GitVersion)
		}
		major, _ = strconv.Atoi(m[1])
		minor, _ = strconv.Atoi(m[2])
	}
	git := info.GitVersion
	if git == "" {
		git = fmt.Sprintf("v%d.%d", major, minor)
	}
	return Tier{Name: classify(major, minor), Major: major, Minor: minor, GitVersion: git}, nil
}

func classify(major, minor int) string {
	switch {
	case major > 1 || (major == 1 && minor >= 22):
		return "full"
	case major == 1 && minor >= 18:
		return "best-effort"
	default:
		return "unsupported"
	}
}

func parseVersionComponent(s string) (int, error) {
	s = strings.TrimSpace(s)
	end := 0
	for end < len(s) && s[end] >= '0' && s[end] <= '9' {
		end++
	}
	if end == 0 {
		return 0, fmt.Errorf("no leading digits in %q", s)
	}
	return strconv.Atoi(s[:end])
}
