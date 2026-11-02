// Package output renders command results as a table, JSON or YAML.
package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"go.yaml.in/yaml/v3"
)

// Mode selects the renderer. It is the value of -o and of LFED_OUTPUT.
type Mode string

// Supported modes.
const (
	Table Mode = "table"
	JSON  Mode = "json"
	YAML  Mode = "yaml"
)

// Modes lists the -o values, for help text.
const Modes = "table, json or yaml"

// ParseMode validates a -o value.
func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case Table, JSON, YAML:
		return Mode(s), nil
	case "":
		return Table, nil
	}
	return "", fmt.Errorf("invalid output %q; expected %s", s, Modes)
}

// Structured reports whether mode renders the value itself rather than
// the table columns.
func (m Mode) Structured() bool { return m == JSON || m == YAML }

// Column describes one table column over rows of type T.
type Column[T any] struct {
	Header string
	Value  func(T) string
}

// Print renders rows. Table mode prints a header line and one line per
// row. JSON and YAML modes marshal rows verbatim as an array; a nil slice
// renders as [].
func Print[T any](w io.Writer, mode Mode, rows []T, columns []Column[T]) error {
	if mode.Structured() {
		if rows == nil {
			rows = []T{}
		}
		return PrintValue(w, mode, rows)
	}
	return printTable(w, rows, columns)
}

// PrintOne renders a single value. JSON and YAML modes marshal v as an
// object; table mode prints a one-row table.
func PrintOne[T any](w io.Writer, mode Mode, v T, columns []Column[T]) error {
	if mode.Structured() {
		return PrintValue(w, mode, v)
	}
	return printTable(w, []T{v}, columns)
}

// PrintValue writes v as JSON or YAML according to mode. YAML goes through
// the JSON encoding first so struct tags and omitempty apply to both. JSON
// is valid YAML, so the text is re-parsed as a yaml.Node, which keeps the
// field order a plain map would lose.
func PrintValue(w io.Writer, mode Mode, v any) error {
	if mode != YAML {
		return PrintJSON(w, v)
	}
	raw, err := json.Marshal(v)
	if err != nil {
		return err
	}
	var node yaml.Node
	if err := yaml.Unmarshal(raw, &node); err != nil {
		return err
	}
	blockStyle(&node)
	enc := yaml.NewEncoder(w)
	enc.SetIndent(2)
	if err := enc.Encode(&node); err != nil {
		return err
	}
	return enc.Close()
}

// PrintJSON writes v as indented JSON followed by a newline.
func PrintJSON(w io.Writer, v any) error {
	enc := json.NewEncoder(w)
	enc.SetEscapeHTML(false)
	enc.SetIndent("", "  ")
	return enc.Encode(v)
}

func printTable[T any](w io.Writer, rows []T, columns []Column[T]) error {
	tw := tabwriter.NewWriter(w, 0, 0, 2, ' ', 0)
	headers := make([]string, len(columns))
	for i, c := range columns {
		headers[i] = strings.ToUpper(c.Header)
	}
	fmt.Fprintln(tw, strings.Join(headers, "\t"))
	cells := make([]string, len(columns))
	for _, row := range rows {
		for i, c := range columns {
			cells[i] = c.Value(row)
		}
		fmt.Fprintln(tw, strings.Join(cells, "\t"))
	}
	return tw.Flush()
}

// blockStyle clears the flow and quoting styles the JSON text carried so
// the encoder emits conventional block YAML.
func blockStyle(n *yaml.Node) {
	n.Style = 0
	for _, c := range n.Content {
		blockStyle(c)
	}
}
