package output

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"text/tabwriter"

	"go.yaml.in/yaml/v3"
)

type Mode string

const (
	Table Mode = "table"
	JSON  Mode = "json"
	YAML  Mode = "yaml"
)

const Modes = "table, json or yaml"

func ParseMode(s string) (Mode, error) {
	switch Mode(s) {
	case Table, JSON, YAML:
		return Mode(s), nil
	case "":
		return Table, nil
	}
	return "", fmt.Errorf("invalid output %q; expected %s", s, Modes)
}

func (m Mode) Structured() bool { return m == JSON || m == YAML }

type Column[T any] struct {
	Header string
	Value  func(T) string
}

func Print[T any](w io.Writer, mode Mode, rows []T, columns []Column[T]) error {
	if mode.Structured() {
		if rows == nil {
			rows = []T{}
		}
		return PrintValue(w, mode, rows)
	}
	return printTable(w, rows, columns)
}

func PrintOne[T any](w io.Writer, mode Mode, v T, columns []Column[T]) error {
	if mode.Structured() {
		return PrintValue(w, mode, v)
	}
	return printTable(w, []T{v}, columns)
}

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

func blockStyle(n *yaml.Node) {
	n.Style = 0
	for _, c := range n.Content {
		blockStyle(c)
	}
}
