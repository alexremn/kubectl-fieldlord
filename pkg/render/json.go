package render

import (
	"encoding/json"
	"io"
)

// JSON writes v as indented JSON followed by a newline. All types are
// structs/slices (no maps), so output is deterministic.
func JSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}
