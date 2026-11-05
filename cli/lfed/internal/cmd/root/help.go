package root

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/pluginmgr"
)

const pluginsFunc = "lfedPlugins"

func usageTemplate(cmd *cobra.Command) string {
	cobra.AddTemplateFunc(pluginsFunc, pluginsSection)
	section := "{{if not .HasParent}}{{" + pluginsFunc + "}}{{end}}"
	tmpl := cmd.UsageTemplate()
	anchor := "{{if .HasAvailableLocalFlags}}"
	if i := strings.Index(tmpl, anchor); i >= 0 {
		return tmpl[:i] + section + tmpl[i:]
	}
	return tmpl + section
}

func pluginsSection() string {
	installed, err := pluginmgr.List()
	if err != nil {
		return ""
	}
	var b strings.Builder

	tw := tabwriter.NewWriter(&b, 14, 0, 1, ' ', 0)
	n := 0
	for _, p := range installed {
		if p.Manifest.Hidden {
			continue
		}
		fmt.Fprintf(tw, "  %s\t%s\n", p.Manifest.Name, p.Manifest.Summary)
		n++
	}
	if n == 0 {
		return ""
	}
	tw.Flush()
	return "\n\nPlugins:\n" + strings.TrimSuffix(b.String(), "\n")
}
