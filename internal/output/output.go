package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"golang.org/x/term"
	"gopkg.in/yaml.v3"
)

type Format string

const (
	FormatTable Format = "table"
	FormatJSON  Format = "json"
	FormatYAML  Format = "yaml"
	FormatAuto  Format = "auto"
)

// DetectFormat resolves the output format. If override is set, use it.
// Otherwise, use table when stdout is a TTY, JSON when piped.
func DetectFormat(override string, stdout *os.File) Format {
	switch strings.ToLower(override) {
	case "table":
		return FormatTable
	case "json":
		return FormatJSON
	case "yaml":
		return FormatYAML
	}
	// Auto-detect
	if term.IsTerminal(int(stdout.Fd())) {
		return FormatTable
	}
	return FormatJSON
}

// Render writes data to w in the requested format.
// data must be a slice of structs or a single struct/map.
// columns is used only for table format; if nil, all exported fields are used.
func Render(w io.Writer, format Format, data interface{}, columns []string) error {
	switch format {
	case FormatJSON:
		enc := json.NewEncoder(w)
		enc.SetIndent("", "  ")
		return enc.Encode(data)
	case FormatYAML:
		return yaml.NewEncoder(w).Encode(data)
	case FormatTable:
		return renderTable(w, data, columns)
	default:
		return fmt.Errorf("unknown format: %s", format)
	}
}

// renderTable prints a kubectl-style columnar table: left-aligned, space-separated,
// no borders, no cell wrapping. Each row is always exactly one line.
func renderTable(w io.Writer, data interface{}, columns []string) error {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	if v.Kind() == reflect.Slice {
		if v.Len() == 0 {
			fmt.Fprintln(w, "(none)")
			return nil
		}
		elem := v.Index(0)
		if elem.Kind() == reflect.Ptr {
			elem = elem.Elem()
		}
		headers, fields := resolveColumns(elem.Type(), columns)

		// Collect all rows as strings so we can compute column widths.
		rows := make([][]string, v.Len())
		for i := 0; i < v.Len(); i++ {
			row := v.Index(i)
			if row.Kind() == reflect.Ptr {
				row = row.Elem()
			}
			rows[i] = extractRow(row, fields)
		}

		printColumns(w, headers, rows)
		return nil
	}

	if v.Kind() == reflect.Struct {
		t := v.Type()
		var rows [][]string
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			rows = append(rows, []string{f.Name, fmt.Sprintf("%v", v.Field(i).Interface())})
		}
		printColumns(w, nil, rows)
		return nil
	}

	fmt.Fprintln(w, data)
	return nil
}

// printColumns writes a kubectl-style table. headers may be nil (for key/value structs).
// Column widths are computed from all data so no cell is ever truncated or wrapped.
func printColumns(w io.Writer, headers []string, rows [][]string) {
	if len(rows) == 0 {
		return
	}
	cols := len(rows[0])

	// Compute max width per column.
	widths := make([]int, cols)
	if headers != nil {
		for i, h := range headers {
			widths[i] = len(h)
		}
	}
	for _, row := range rows {
		for i, cell := range row {
			if len(cell) > widths[i] {
				widths[i] = len(cell)
			}
		}
	}

	printRow := func(cells []string) {
		for i, cell := range cells {
			if i > 0 {
				fmt.Fprint(w, "  ")
			}
			// Last column: no trailing padding needed.
			if i == len(cells)-1 {
				fmt.Fprint(w, cell)
			} else {
				fmt.Fprintf(w, "%-*s", widths[i], cell)
			}
		}
		fmt.Fprintln(w)
	}

	if headers != nil {
		printRow(headers)
	}
	for _, row := range rows {
		printRow(row)
	}
}

func resolveColumns(t reflect.Type, override []string) (headers []string, fields []string) {
	if len(override) > 0 {
		return override, override
	}
	for i := 0; i < t.NumField(); i++ {
		f := t.Field(i)
		if !f.IsExported() {
			continue
		}
		tag := f.Tag.Get("json")
		name := f.Name
		if tag != "" && tag != "-" {
			name = strings.Split(tag, ",")[0]
		}
		headers = append(headers, strings.ToUpper(name))
		fields = append(fields, f.Name)
	}
	return
}

func extractRow(v reflect.Value, fields []string) []string {
	row := make([]string, len(fields))
	t := v.Type()
	for i, fieldName := range fields {
		for j := 0; j < t.NumField(); j++ {
			if t.Field(j).Name == fieldName || strings.Split(t.Field(j).Tag.Get("json"), ",")[0] == fieldName {
				row[i] = fmt.Sprintf("%v", v.Field(j).Interface())
				break
			}
		}
	}
	return row
}
