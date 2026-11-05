// Package pluginmgr manages installed lfed plugins.
//
// A plugin lives in $XDG_DATA_HOME/lfed/plugins/<name>/ as the binary
// lfed-<name> and the manifest lfed-plugin.json. the lfed CLI reads manifests
// for listing and help. It executes a plugin only to run it, to complete
// its arguments, or to read its manifest at install time.
package pluginmgr

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"sort"
	"strings"

	"github.com/Masterminds/semver/v3"

	sdkplugin "github.com/LF-Certification/p3-forge/cli/lfed/sdk/plugin"
)

// ManifestFile is the manifest name inside a plugin directory.
const ManifestFile = "lfed-plugin.json"

// Installed is one plugin found in the plugin directory.
type Installed struct {
	Manifest sdkplugin.Manifest
	// Source is where the plugin came from. Zero when lfed-plugin.json
	// records no source.
	Source Source
	// Dir is the plugin directory: <plugins>/<name>.
	Dir string
	// Binary is the executable path: <plugins>/<name>/lfed-<name>.
	Binary string
}

// record is the on-disk shape of lfed-plugin.json: the manifest the plugin
// printed plus the source the lfed CLI installed it from.
type record struct {
	sdkplugin.Manifest
	Source Source `json:"source"`
}

// NotFoundError reports a word that is neither a core command nor an
// installed plugin. The lfed CLI exits 2. Suggestions, when the caller
// filled them, are close command words.
type NotFoundError struct {
	Name        string
	Suggestions []string
}

func (e *NotFoundError) Error() string {
	if len(e.Suggestions) > 0 {
		return fmt.Sprintf("unknown command %q for \"lfed\"; did you mean %s?", e.Name, strings.Join(e.Suggestions, ", "))
	}
	return fmt.Sprintf("unknown command %q for \"lfed\"; run \"lfed plugin list\"", e.Name)
}

// IncompatibleError reports a plugin whose requires_lfed range excludes the
// running lfed CLI. the lfed CLI exits 2.
type IncompatibleError struct {
	Name     string
	Requires string
	Running  string
}

func (e *IncompatibleError) Error() string {
	return fmt.Sprintf("plugin %q requires lfed %s; the lfed CLI is %s", e.Name, e.Requires, e.Running)
}

// Dir returns the plugin directory: LFED_PLUGIN_DIR when set, else
// $XDG_DATA_HOME/lfed/plugins, else ~/.local/share/lfed/plugins.
// LFED_PLUGIN_DIR is a supported override, for example for a shared or
// read-only plugin set.
func Dir() (string, error) {
	if d := os.Getenv("LFED_PLUGIN_DIR"); d != "" {
		return d, nil
	}
	if xdg := os.Getenv("XDG_DATA_HOME"); xdg != "" {
		return filepath.Join(xdg, "lfed", "plugins"), nil
	}
	home, err := os.UserHomeDir()
	if err != nil {
		return "", fmt.Errorf("resolve plugin dir: %w", err)
	}
	return filepath.Join(home, ".local", "share", "lfed", "plugins"), nil
}

// BinaryName returns the executable name of a plugin.
func BinaryName(name string) string {
	return "lfed-" + name
}

// List returns installed plugins sorted by name. It reads manifests
// only. Directories without a manifest, and directories whose name is not
// a plugin name (such as .install-* staging and .prev-* backups), are
// skipped.
func List() ([]Installed, error) {
	dir, err := Dir()
	if err != nil {
		return nil, err
	}
	entries, err := os.ReadDir(dir)
	if errors.Is(err, os.ErrNotExist) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("read plugin dir: %w", err)
	}
	var out []Installed
	for _, e := range entries {
		if !e.IsDir() || !nameRe.MatchString(e.Name()) {
			continue
		}
		p, err := load(dir, e.Name())
		if errors.Is(err, os.ErrNotExist) {
			continue
		}
		if err != nil {
			return nil, err
		}
		out = append(out, p)
	}
	sort.Slice(out, func(i, j int) bool { return out[i].Manifest.Name < out[j].Manifest.Name })
	return out, nil
}

// Lookup returns the plugin called name. A name that does not match nameRe
// or a missing binary yields *NotFoundError. A binary without a manifest,
// or a manifest of another schema_version, yields an error that asks for
// a reinstall.
func Lookup(name string) (Installed, error) {
	if !nameRe.MatchString(name) {
		return Installed{}, &NotFoundError{Name: name}
	}
	dir, err := Dir()
	if err != nil {
		return Installed{}, err
	}
	bin := filepath.Join(dir, name, BinaryName(name))
	if _, err := os.Stat(bin); err != nil {
		return Installed{}, &NotFoundError{Name: name}
	}
	p, err := load(dir, name)
	if errors.Is(err, os.ErrNotExist) {
		return Installed{}, fmt.Errorf("plugin %q has no %s; run \"lfed plugin install\" again", name, ManifestFile)
	}
	if err != nil {
		return Installed{}, err
	}
	if p.Manifest.SchemaVersion != sdkplugin.ManifestSchemaVersion {
		return Installed{}, fmt.Errorf("plugin %q has manifest schema_version %q; this lfed reads %q; run \"lfed plugin install\" again",
			name, p.Manifest.SchemaVersion, sdkplugin.ManifestSchemaVersion)
	}
	return p, nil
}

// load reads <dir>/<name>/lfed-plugin.json. Missing files return an error
// wrapping os.ErrNotExist.
func load(dir, name string) (Installed, error) {
	pdir := filepath.Join(dir, name)
	data, err := os.ReadFile(filepath.Join(pdir, ManifestFile))
	if err != nil {
		return Installed{}, err
	}
	var r record
	if err := json.Unmarshal(data, &r); err != nil {
		return Installed{}, fmt.Errorf("parse %s: %w", filepath.Join(pdir, ManifestFile), err)
	}
	return Installed{Manifest: r.Manifest, Source: r.Source, Dir: pdir, Binary: filepath.Join(pdir, BinaryName(name))}, nil
}

// CheckLfedVersion verifies that the running lfed version satisfies the
// requires_lfed range. A pre-release tag such as 0.1.0-dev is ignored so development
// builds pass release ranges. Failure yields *IncompatibleError; the name
// is filled by the caller when it is known.
func CheckLfedVersion(requires, running string) error {
	c, err := semver.NewConstraint(requires)
	if err != nil {
		return fmt.Errorf("invalid requires_lfed %q: %w", requires, err)
	}
	v, err := semver.NewVersion(running)
	if err != nil {
		return fmt.Errorf("lfed version %q is not semver: %w", running, err)
	}
	release, err := v.SetPrerelease("")
	if err != nil {
		return err
	}
	if !c.Check(&release) {
		return &IncompatibleError{Requires: requires, Running: running}
	}
	return nil
}
