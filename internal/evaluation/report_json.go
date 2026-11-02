package evaluation

import (
	"encoding/json"
	"fmt"
	"io"
)

// WriteJSON renders the report as one indented JSON document, for scripts
// and for comparing runs.
//
//	err := report.WriteJSON(os.Stdout)
func (r Report) WriteJSON(w io.Writer) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	if err := enc.Encode(r); err != nil {
		return fmt.Errorf("write json report: %w", err)
	}
	return nil
}
