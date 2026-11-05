package pluginmgr

import (
	"context"
	"fmt"

	"github.com/Masterminds/semver/v3"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
)

type UpgradeResult struct {
	Name string `json:"name"`
	From string `json:"from"`

	To       string `json:"to,omitempty"`
	Upgraded bool   `json:"upgraded"`

	Note string `json:"note,omitempty"`
}

func Upgrade(ctx context.Context, f *cmdutil.Factory, names []string, dryRun bool, opts Options) ([]UpgradeResult, error) {
	var plugins []Installed
	if len(names) == 0 {
		var err error
		if plugins, err = List(); err != nil {
			return nil, err
		}
	} else {
		for _, n := range names {
			p, err := Lookup(n)
			if err != nil {
				return nil, err
			}
			plugins = append(plugins, p)
		}
	}
	results := make([]UpgradeResult, 0, len(plugins))
	for _, p := range plugins {
		r, err := upgradeOne(ctx, f, p, dryRun, opts)
		if err != nil {
			return results, fmt.Errorf("%s: %w", p.Manifest.Name, err)
		}
		results = append(results, r)
	}
	return results, nil
}

func upgradeOne(ctx context.Context, f *cmdutil.Factory, p Installed, dryRun bool, opts Options) (UpgradeResult, error) {
	r := UpgradeResult{Name: p.Manifest.Name, From: p.Manifest.Version}
	switch p.Source.Type {
	case SourceOCI:
	case SourcePath:
		r.Note = fmt.Sprintf("installed from %s; reinstall with \"lfed plugin install %s\"", p.Source.Path, p.Source.Path)
		return r, nil
	default:
		r.Note = "source unknown; reinstall with \"lfed plugin install\""
		return r, nil
	}
	if p.Source.Pinned {
		pin := p.Source.Version
		if p.Source.digestPin() {
			pin = p.Source.Digest
		}
		r.Note = fmt.Sprintf("pinned to %s; reinstall to change", pin)
		return r, nil
	}
	want := Source{Type: SourceOCI, Ref: p.Source.Ref}
	repo, err := ociRepository(want)
	if err != nil {
		return r, err
	}
	res, err := resolveOCI(ctx, repo, want, f.Version)
	if err != nil {
		return r, err
	}
	to := res.Version
	r.To = to
	if !newer(to, p.Source.Version) {
		r.Note = "current"
		return r, nil
	}
	if dryRun {
		r.Note = "dry run"
		return r, nil
	}
	want.Version = to
	if _, err := install(ctx, f, want, opts); err != nil {
		return r, err
	}
	r.Upgraded = true
	return r, nil
}

func newer(candidate, installed string) bool {
	c, err1 := semver.NewVersion(candidate)
	i, err2 := semver.NewVersion(installed)
	if err1 != nil || err2 != nil {
		return candidate != installed
	}
	return c.GreaterThan(i)
}
