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

When `cosign` is on `PATH`, the installer also verifies
`checksums.txt.sigstore.json` against the identity
`https://github.com/LF-Certification/p3-forge/.github/workflows/cli-release.yml@refs/tags/cli/lfed/vX.Y.Z`.
A missing or mismatched signature stops the install. Without `cosign`,
the installer prints that it did not check the signature.

From source: `make install-local` builds into `$GOBIN`.

## First use

```bash
lfed context use staging
lfed auth login
lfed plugin install cluster
lfed cluster --help
```

## Credentials

By default, lfed keeps credentials in the system keychain. On a headless
host or in CI, pass `--insecure-storage` to `auth login` to keep them in
the owner-only file `credentials-<context>.json` in the state directory
(`$LFED_STATE_DIR`, else `$XDG_STATE_HOME/lfed`, else
`~/.local/state/lfed`). Use the same flag on later commands. A plugin
started by `lfed --insecure-storage` gets `LFED_INSECURE_STORAGE=1`, and
the `lfed auth token` call of the plugin then uses the file without the
flag. `lfed auth logout` removes the credentials from both the keychain and
the file, so it does not need the flag. A login with the other backend
removes the credentials of the previous login. lfed writes the credential,
state and config files atomically with mode 0600 in directories with mode
0700. The lock file `lock-<context>` in the state directory makes
concurrent lfed processes wait (at most 30 seconds) for each other during
login, logout and token refresh, so a refresh token is redeemed once.

## Plugins

Plugins are published as OCI artifacts under
`ghcr.io/lf-certification/lfed-cli-plugins/<name>`. The packages are
private; the pull needs a GitHub token with `read:packages`:

```bash
gh auth refresh -s read:packages
```

`lfed` looks for credentials in the Docker config (`docker login ghcr.io`),
then `GITHUB_TOKEN`, `GH_TOKEN`, then `gh auth token`. ghcr.io accepts
classic tokens only; a fine-grained token fails with `token provided does
not match expected scopes`.

| Command | Effect |
|---|---|
| `lfed plugin install cluster` | newest version that supports this `lfed` |
| `lfed plugin install cluster@0.2.0` | that version; `plugin upgrade` then skips it until it is reinstalled |
| `lfed plugin install cluster@sha256:<hex>` | that OCI index digest; also skipped by `plugin upgrade` |
| `lfed plugin install ghcr.io/acme/tools/foo:1.0.0` | any registry, full reference |
| `lfed plugin install ./cli` | build and install a local Go main package, or copy a binary |
| `lfed plugin upgrade [name...]` | re-resolve registry installs; `--dry-run` only reports |
| `lfed plugin list` | installed plugins with version, status and source |
| `lfed plugin remove <name>` | delete the plugin directory |

A local path MUST start with `./`, `../`, `/` or `~/`. A bare word is a
plugin name. `LFED_PLUGIN_REGISTRY` replaces the default registry prefix.

Plugins install into `$XDG_DATA_HOME/lfed/plugins/<name>/` (default
`~/.local/share/lfed/plugins`) as `lfed-<name>` plus `lfed-plugin.json`.
`LFED_PLUGIN_DIR` overrides the directory. It is a supported setting, for
example for a per-project plugin set or for tests.

### Signature verification

`lfed plugin install` and `lfed plugin upgrade` from a registry verify the
cosign keyless signature of the plugin artifact before the plugin binary
runs. The signature is a sigstore bundle attached as an OCI referrer of the
index digest. Installs from a local path are never verified.

For the default registry, `lfed` expects the certificate identity
`https://github.com/LF-Certification/p3-platform/.github/workflows/lfed-cli-plugins-release.yml@refs/tags/lfed-cli-<name>/v<version>`
and the issuer `https://token.actions.githubusercontent.com`. The identity
binds the plugin name and version.

For other registries, set both variables, else the install fails:

- `LFED_PLUGIN_SIGNER_IDENTITY`: Go RE2 regular expression for the
  certificate SAN. Anchor it with `^...$`.
- `LFED_PLUGIN_SIGNER_ISSUER`: exact OIDC issuer URL.

Both registries verify against the Sigstore public-good trust root,
fetched through TUF and cached under `$XDG_CACHE_HOME/lfed/sigstore`. The
variables select the signer identity; they do not configure a private
Fulcio or Rekor.

`--allow-unsigned` on `plugin install` and `plugin upgrade` skips the
verification and prints `warning: <ref> installed without signature
verification`.

Rollout: a host with verification refuses plugin versions that have no
signature. Re-release every plugin from the signing release workflow
before you distribute such a host. Until then, operators MUST use
`--allow-unsigned`.

### Global flags

`-c/--context`, `-o/--output`, `--jq`, `--yes` and `--insecure-storage`
are flags of `lfed` itself. Their position decides who reads them:

- Before the plugin name, both forms apply: `lfed -c prod -o json placement targets list`.
- After the plugin name, only the long forms apply, and only before `--`:
  `lfed placement targets list --context prod --output json`. The plugin
  also receives them unchanged and accepts them.
- After the plugin name, `-c` and `-o` belong to the plugin. `lfed`
  ignores them.

### Plugin environment

A plugin inherits the environment of the operator, the same as `gh`,
`kubectl` and `docker` plugins. `lfed` adds these variables. A later value
replaces an exported one:

| Variable | Value |
|---|---|
| `LFED_CLI_VERSION` | semver of `lfed` |
| `LFED_CONTEXT` | active context name |
| `LFED_API_BASE_URL` | https API base URL of the context, no trailing slash |
| `LFED_OUTPUT` | `table`, `json` or `yaml` |
| `LFED_REEXEC` | absolute path of the `lfed` binary |
| `LFED_YES` | `1` when `--yes` was given, else empty |
| `LFED_INSECURE_STORAGE` | `1` when `--insecure-storage` was given, else empty |

`lfed` sets `LFED_YES` and `LFED_INSECURE_STORAGE` to the empty value when
the flag is not given, so a stale exported value has no effect.

`lfed` does not export an access token. A plugin gets a token only when it
needs one: `sdk/auth.Token` runs `$LFED_REEXEC auth token`, which refreshes
the token when necessary. An `LFED_TOKEN` exported in the operator's shell
is inherited like any other variable; the SDK ignores it.

`__complete` gets the same environment. At install time, `__metadata` gets
only `PATH`, `HOME`, `TMPDIR`, `TEMP`, `TMP`, `SystemRoot` (Windows) and
`LFED_CLI_VERSION`.

The SDK (`sdk/plugin.Serve`) registers `--context`, `--jq`, `--yes` and
`--insecure-storage` as hidden flags of the plugin when the plugin does not
define them. When `LFED_YES` is set, it sets every bool `yes` flag of the
plugin to true, on the root and on each command.

## Exit codes

| Code | Meaning |
|---|---|
| 0 | success |
| 1 | error |
| 2 | usage error: unknown flag or command, bad argument count, unknown or incompatible plugin, no compatible release |
| 130 | interrupted by SIGINT or SIGTERM; `lfed` cleans up before it exits |
| 128+N | a plugin was killed by signal N |

Any other code from a plugin passes through unchanged.

While a plugin runs, `lfed` waits for it and never kills it. SIGTERM sent
to `lfed` is relayed to the plugin. SIGINT is not relayed: the terminal
delivers it to `lfed` and the plugin at the same time, and a second SIGINT
would make tools such as terraform force-quit. A second Ctrl-C ends `lfed`
with the default action; what the plugin does with it is the plugin's
choice.

In an SDK plugin (`sdk/plugin.Serve`), the first SIGINT or SIGTERM cancels
the command context. A command that returns within `plugin.CancelGrace`
(3 s) sets the exit code; a command that does not watch the context, for
example one blocked in a prompt, is ended with exit 130 when the grace
runs out. A second signal ends the plugin at once.

## Versioning

Releases are tagged `cli/lfed/vX.Y.Z` and built by
`.github/workflows/cli-release.yml`. See `VERSIONING.md` at the repository
root. The Go module `github.com/LF-Certification/p3-forge/cli/lfed` is
`v0.x`: the `sdk/` packages MAY change between minor versions. Plugins pin
the SDK version in `go.mod` and declare the host range they accept in the
manifest field `requires_lfed`.

## Development

```bash
make lint test          # vet, gofmt, staticcheck, gosec, govulncheck, go test
make build              # bin/lfed with a 0.1.0-dev+<sha> version
make release-snapshot   # GoReleaser archives in dist/, no publish
```
