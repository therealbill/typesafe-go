package cli

import (
	"encoding/json"
	"io"
)

// IO bundles the streams a command reads and writes.
type IO struct {
	In  io.Reader
	Out io.Writer
	Err io.Writer
}

// writeJSON encodes v to w as one JSON document followed by a newline.
func writeJSON(w io.Writer, v any, pretty bool) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	if pretty {
		enc.SetIndent("", "  ")
	}
	return enc.Encode(v)
}
