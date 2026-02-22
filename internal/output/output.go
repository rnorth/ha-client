package output

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"reflect"
	"strings"

	"github.com/olekukonko/tablewriter"
	"github.com/olekukonko/tablewriter/tw"
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

func renderTable(w io.Writer, data interface{}, columns []string) error {
	v := reflect.ValueOf(data)
	if v.Kind() == reflect.Ptr {
		v = v.Elem()
	}

	// Handle slice of structs
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

		table := tablewriter.NewTable(w,
			tablewriter.WithBorders(tw.Border{Left: tw.Off, Right: tw.Off, Top: tw.Off, Bottom: tw.Off}),
			tablewriter.WithHeaderAlignment(tw.AlignLeft),
		)
		table.Header(toAnySlice(headers)...)
		for i := 0; i < v.Len(); i++ {
			row := v.Index(i)
			if row.Kind() == reflect.Ptr {
				row = row.Elem()
			}
			table.Append(toAnySlice(extractRow(row, fields))...)
		}
		return table.Render()
	}

	// Single struct
	if v.Kind() == reflect.Struct {
		table := tablewriter.NewTable(w,
			tablewriter.WithBorders(tw.Border{Left: tw.Off, Right: tw.Off, Top: tw.Off, Bottom: tw.Off}),
		)
		t := v.Type()
		for i := 0; i < v.NumField(); i++ {
			f := t.Field(i)
			if !f.IsExported() {
				continue
			}
			table.Append(f.Name, fmt.Sprintf("%v", v.Field(i).Interface()))
		}
		return table.Render()
	}

	// Fallback
	fmt.Fprintln(w, data)
	return nil
}

func toAnySlice(ss []string) []interface{} {
	out := make([]interface{}, len(ss))
	for i, s := range ss {
		out[i] = s
	}
	return out
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
