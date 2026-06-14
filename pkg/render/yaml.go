package render

import (
	"io"

	"sigs.k8s.io/yaml"
)

// YAML writes v as YAML. sigs.k8s.io/yaml marshals via JSON tags, so the keys
// match the JSON envelope exactly.
func YAML(w io.Writer, v any) error {
	b, err := yaml.Marshal(v)
	if err != nil {
		return err
	}
	_, err = w.Write(b)
	return err
}
