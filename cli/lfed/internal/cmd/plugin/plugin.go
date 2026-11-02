// Package plugin implements `lfed plugin install|list|remove`.
package plugin

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/pluginmgr"
	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
	sdkplugin "github.com/LF-Certification/p3-forge/cli/lfed/sdk/plugin"
)

// NewCmd builds the `lfed plugin` group.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "plugin",
		Short: "Install, list, upgrade and remove plugins",
	}
	cmd.AddCommand(newInstallCmd(f), newListCmd(f), newUpgradeCmd(f), newRemoveCmd(f))
	return cmd
}

func newInstallCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "install <name>[@<version>] | <registry>/<repository>[:<version>] | ./<path>",
		Short: "Install a plugin from the plugin registry or a local path",
		Long: `Install a plugin.

A bare <name> resolves to ` + pluginmgr.DefaultRegistry + `/<name> and
installs the highest version that supports this lfed. <name>@<version>
pins a version. A full OCI reference works for any registry. Set
LFED_PLUGIN_REGISTRY to change the default registry.

A local path MUST start with ./, ../, / or ~/. It is an executable file,
or a directory holding a Go main package directly or under cmd/lfed-*;
a directory is built with "go build".

The lfed CLI runs "<binary> __metadata", validates the manifest and stores
the binary, the manifest and the source under the plugin directory. An
existing plugin of the same name is replaced.

ghcr.io credentials come from the Docker config, GITHUB_TOKEN, GH_TOKEN,
or "gh auth token". The token needs the read:packages scope:
"gh auth refresh -s read:packages".`,
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			p, err := pluginmgr.Install(cmd.Context(), f, args[0], pluginmgr.Options{Reserved: reservedNames(cmd.Root())})
			if err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.Out, "installed %s %s from %s\n", p.Manifest.Name, p.Manifest.Version, p.Source)
			return nil
		},
	}
}

// reservedNames are the core command words plus the ones Cobra adds at
// Execute time. A plugin with one of these names is unreachable.
func reservedNames(root *cobra.Command) []string {
	names := []string{"help", "completion"}
	for _, c := range root.Commands() {
		names = append(names, c.Name())
	}
	return names
}

// listRow is one `plugin list` entry: the manifest, its source and the
// compatibility check.
type listRow struct {
	sdkplugin.Manifest
	Source pluginmgr.Source `json:"source"`
	// Compatible is false when requires_lfed rejects this lfed version.
	Compatible bool `json:"compatible"`
}

func (r listRow) status() string {
	if r.Compatible {
		return "ok"
	}
	return "needs lfed " + r.RequiresLfed
}

func newListCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "List installed plugins",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := f.Output()
			if err != nil {
				return err
			}
			installed, err := pluginmgr.List()
			if err != nil {
				return err
			}
			rows := make([]listRow, 0, len(installed))
			for _, p := range installed {
				rows = append(rows, listRow{
					Manifest:   p.Manifest,
					Source:     p.Source,
					Compatible: pluginmgr.CheckLfedVersion(p.Manifest.RequiresLfed, f.Version) == nil,
				})
			}
			return output.Print(f.IOStreams.Out, mode, rows, []output.Column[listRow]{
				{Header: "name", Value: func(r listRow) string { return r.Name }},
				{Header: "version", Value: func(r listRow) string { return r.Version }},
				{Header: "status", Value: listRow.status},
				{Header: "source", Value: func(r listRow) string { return r.Source.String() }},
				{Header: "summary", Value: func(r listRow) string { return r.Summary }},
			})
		},
	}
}

func newRemoveCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "remove <name>",
		Short: "Remove an installed plugin",
		Args:  cobra.ExactArgs(1),
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := pluginmgr.Remove(args[0]); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.Out, "removed %s\n", args[0])
			return nil
		},
	}
}

func newUpgradeCmd(f *cmdutil.Factory) *cobra.Command {
	var dryRun bool
	cmd := &cobra.Command{
		Use:   "upgrade [name...]",
		Short: "Upgrade installed plugins to the newest version this lfed supports",
		Long: `Upgrade installed plugins.

Each plugin installed from the registry is re-resolved against the
repository recorded at install time, and replaced when a newer version
supports this lfed. Plugins installed from a local path are listed and
left alone. With no names, every installed plugin is checked.`,
		ValidArgsFunction: func(*cobra.Command, []string, string) ([]string, cobra.ShellCompDirective) {
			return Names(), cobra.ShellCompDirectiveNoFileComp
		},
		RunE: func(cmd *cobra.Command, args []string) error {
			mode, err := f.Output()
			if err != nil {
				return err
			}
			results, err := pluginmgr.Upgrade(cmd.Context(), f, args, dryRun, pluginmgr.Options{Reserved: reservedNames(cmd.Root())})
			if err != nil {
				return err
			}
			return output.Print(f.IOStreams.Out, mode, results, []output.Column[pluginmgr.UpgradeResult]{
				{Header: "name", Value: func(r pluginmgr.UpgradeResult) string { return r.Name }},
				{Header: "from", Value: func(r pluginmgr.UpgradeResult) string { return r.From }},
				{Header: "to", Value: func(r pluginmgr.UpgradeResult) string { return r.To }},
				{Header: "result", Value: upgradeResult},
			})
		},
	}
	cmd.Flags().BoolVar(&dryRun, "dry-run", false, "Resolve versions without installing")
	return cmd
}

func upgradeResult(r pluginmgr.UpgradeResult) string {
	if r.Upgraded {
		return "upgraded"
	}
	return r.Note
}

// Names returns "name\tsummary" completions for installed plugins. Hidden
// plugins are omitted. Errors yield no completions.
func Names() []string {
	installed, err := pluginmgr.List()
	if err != nil {
		return nil
	}
	names := make([]string, 0, len(installed))
	for _, p := range installed {
		if p.Manifest.Hidden {
			continue
		}
		names = append(names, p.Manifest.Name+"\t"+p.Manifest.Summary)
	}
	return names
}
