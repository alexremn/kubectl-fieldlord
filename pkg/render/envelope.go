// Package render serializes ownership/drift results to JSON, YAML, or a table.
package render

// SchemaVersion is the version tag of the JSON/YAML output envelope.
const SchemaVersion = "v1"

// ResourceRef identifies the object a result describes.
type ResourceRef struct {
	Group     string `json:"group"`
	Version   string `json:"version"`
	Kind      string `json:"kind"`
	Namespace string `json:"namespace,omitempty"`
	Name      string `json:"name"`
}

// Envelope is the stable top-level shape of json/yaml output. Findings holds a
// command-specific slice ([]ownership.OwnedPath or []drift.Finding).
type Envelope struct {
	SchemaVersion string      `json:"schemaVersion"`
	Command       string      `json:"command"`
	Resource      ResourceRef `json:"resource"`
	ServerVersion string      `json:"serverVersion,omitempty"`
	SupportTier   string      `json:"supportTier,omitempty"`
	Findings      any         `json:"findings"`
	Warnings      []string    `json:"warnings"`
}
