package oden

import (
	"encoding/json"
	"io"
	"strings"
	"text/tabwriter"
)

// WriteJSON encodes v as 2-space indented JSON followed by a newline.
func WriteJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetIndent("", "  ")
	enc.SetEscapeHTML(false)
	return enc.Encode(v)
}

// WriteTable writes an aligned table with uppercase headers and
// newline/tab-sanitized cells using text/tabwriter.
func WriteTable(w io.Writer, headers []string, rows [][]string) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	if len(headers) > 0 {
		upper := make([]string, len(headers))
		for i, h := range headers {
			upper[i] = strings.ToUpper(sanitizeCell(h))
		}
		if _, err := io.WriteString(tw, strings.Join(upper, "\t")+"\n"); err != nil {
			return err
		}
	}
	for _, row := range rows {
		clean := make([]string, len(row))
		for i, cell := range row {
			clean[i] = sanitizeCell(cell)
		}
		if _, err := io.WriteString(tw, strings.Join(clean, "\t")+"\n"); err != nil {
			return err
		}
	}
	return tw.Flush()
}

var cellSanitizer = strings.NewReplacer("\r\n", " ", "\r", " ", "\n", " ", "\t", " ")

func sanitizeCell(s string) string {
	return cellSanitizer.Replace(s)
}

// WriteJSON encodes v as indented JSON to the command's stdout.
func (c *Ctx) WriteJSON(v any) error {
	return WriteJSON(c.Stdout, v)
}

// WriteTable writes an aligned table to the command's stdout.
func (c *Ctx) WriteTable(headers []string, rows [][]string) error {
	return WriteTable(c.Stdout, headers, rows)
}
