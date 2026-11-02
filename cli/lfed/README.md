# lfed

`lfed` is the education platform CLI. The host binary provides contexts,
login and a plugin manager. Plugins add the operator commands and live in
other repositories.

## Install

```bash
curl -fsSL https://raw.githubusercontent.com/LF-Certification/p3-forge/main/cli/lfed/install.sh | bash
```

The installer downloads the newest `cli/lfed/v*` GitHub Release for your
OS and architecture, verifies `checksums.txt`, and installs `lfed` into
`~/.local/bin`. Set `LFED_VERSION=X.Y.Z` to pin a version and
`LFED_INSTALL_DIR` to change the directory.

From source: `make install-local` builds into `$GOBIN`.

## First use

```bash
lfed context use staging
lfed auth login
lfed plugin install cluster
lfed cluster --help
```

## Plugins

Plugins are published as OCI artifacts under
`ghcr.io/lf-certification/lfed-cli-plugins/<name>`. The packages are
private; the pull needs a GitHub token with `read:packages`:

```bash
gh auth refresh -s read:packages
```

`lfed` looks for credentials in the Docker config (`docker login ghcr.io`),
then `GITHUB_TOKEN`, `GH_TOKEN`, then `gh auth token`.

| Command | Effect |
|---|---|
| `lfed plugin install cluster` | newest version that supports this `lfed` |
| `lfed plugin install cluster@0.2.0` | that version |
| `lfed plugin install ghcr.io/acme/tools/foo:1.0.0` | any registry, full reference |
| `lfed plugin install ./cli` | build and install a local Go main package, or copy a binary |
| `lfed plugin upgrade [name...]` | re-resolve registry installs; `--dry-run` only reports |
| `lfed plugin list` | installed plugins with version, status and source |
| `lfed plugin remove <name>` | delete the plugin directory |

A local path MUST start with `./`, `../`, `/` or `~/`. A bare word is a
plugin name. `LFED_PLUGIN_REGISTRY` replaces the default registry prefix.

Plugins install into `$XDG_DATA_HOME/lfed/plugins/<name>/` (default
`~/.local/share/lfed/plugins`) as `lfed-<name>` plus `lfed-plugin.json`.
`LFED_PLUGIN_DIR` overrides the directory.

## Versioning

Releases are tagged `cli/lfed/vX.Y.Z` and built by
`.github/workflows/cli-release.yml`. See `VERSIONING.md` at the repository
root. The Go module `github.com/LF-Certification/p3-forge/cli/lfed` is
`v0.x`: the `sdk/` packages MAY change between minor versions. Plugins pin
the SDK version in `go.mod` and declare the host range they accept in the
manifest field `requires_lfed`.

## Development

```bash
make lint test          # vet, gofmt, govulncheck, go test
make build              # bin/lfed with a 0.1.0-dev+<sha> version
make release-snapshot   # GoReleaser archives in dist/, no publish
```
