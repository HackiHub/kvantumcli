package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
)

// JSON pretty-prints v as indented JSON to stdout.
func JSON(v any) error {
	return JSONTo(os.Stdout, v)
}

// JSONTo pretty-prints v as indented JSON to w.
func JSONTo(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

// Error writes a human-readable error message to stderr.
func Error(err error) {
	if err == nil {
		return
	}
	fmt.Fprintf(os.Stderr, "error: %v\n", err)
}
