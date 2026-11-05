package root

import (
	"fmt"
	"strings"
	"text/tabwriter"

	"github.com/spf13/cobra"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/pluginmgr"
)

// pluginsFunc is the template function name used by usageTemplate.
const pluginsFunc = "lfedPlugins"

// usageTemplate returns the Cobra default usage template with a Plugins
// section before Flags. The section renders only for the root command.
// When the anchor is missing after a Cobra upgrade the section is appended
// so help never loses the plugin list.
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

// pluginsSection renders installed, non-hidden plugins from manifests. It
// returns "" when none are installed or the plugin dir is unreadable, so
// help output never fails.
func pluginsSection() string {
	installed, err := pluginmgr.List()
	if err != nil {
		return ""
	}
	var b strings.Builder
	// minwidth 14 is two indent spaces, NamePadding (11) and one space.
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
