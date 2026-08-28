package cli

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"

	"github.com/connorfranc/magicite/internal/client"
	"github.com/connorfranc/magicite/internal/wire"
)

// EmitJSON writes data in the stable magicite output envelope.
func EmitJSON(w io.Writer, kind string, data any) error {
	return json.NewEncoder(w).Encode(struct {
		Schema int    `json:"schema"`
		Kind   string `json:"kind"`
		Data   any    `json:"data"`
	}{Schema: wire.Schema, Kind: kind, Data: data})
}

// EmitTable writes an uppercase, space-aligned table.
func EmitTable(w io.Writer, headers []string, rows [][]string) error {
	widths := make([]int, len(headers))
	for i, header := range headers {
		widths[i] = len(strings.ToUpper(header))
	}
	for _, row := range rows {
		for i, value := range row {
			if i < len(widths) && len(value) > widths[i] {
				widths[i] = len(value)
			}
		}
	}
	writeRow := func(values []string) error {
		for i := range widths {
			if i > 0 {
				if _, err := io.WriteString(w, "  "); err != nil {
					return err
				}
			}
			value := ""
			if i < len(values) {
				value = values[i]
			}
			if _, err := fmt.Fprintf(w, "%-*s", widths[i], value); err != nil {
				return err
			}
		}
		_, err := io.WriteString(w, "\n")
		return err
	}
	header := make([]string, len(headers))
	for i, value := range headers {
		header[i] = strings.ToUpper(value)
	}
	if err := writeRow(header); err != nil {
		return err
	}
	for _, row := range rows {
		if err := writeRow(row); err != nil {
			return err
		}
	}
	return nil
}

// EmitLine writes one formatted line.
func EmitLine(w io.Writer, format string, args ...any) error {
	_, err := fmt.Fprintf(w, format+"\n", args...)
	return err
}

// Fail reports one command error and returns its documented exit status.
func Fail(e *Env, err error) int {
	_, _ = fmt.Fprintf(e.Err, "magicite: %v\n", err)
	return client.ExitStatus(err)
}
