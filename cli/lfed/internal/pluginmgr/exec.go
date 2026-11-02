package pluginmgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"slices"
	"strings"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/contexts"
	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

// ExitError carries a plugin's non-zero exit code. main exits with Code
// and prints nothing; the plugin already reported the failure.
type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("plugin exited with code %d", e.Code)
}

// ExitCode maps an error to the lfed CLI exit code: 0 for nil, the plugin's
// code for *ExitError, 2 for *NotFoundError and *IncompatibleError, else 1.
func ExitCode(err error) int {
	if err == nil {
		return 0
	}
	var exit *ExitError
	if errors.As(err, &exit) {
		return exit.Code
	}
	var notFound *NotFoundError
	var incompatible *IncompatibleError
	var noRelease *NoReleaseError
	if errors.As(err, &notFound) || errors.As(err, &incompatible) || errors.As(err, &noRelease) {
		return 2
	}
	return 1
}

// Env returns the six LFED_* variables of the plugin contract. The base URL
// has no trailing slash; contexts.Context.APIBaseURL guarantees that.
// LFED_DEBUG is not set here: it passes through from the operator's
// environment, so a plugin sees exactly what the CLI saw.
func Env(lfedVersion string, ctx contexts.Context, token string, mode output.Mode, reexec string) []string {
	return []string{
		"LFED_CLI_VERSION=" + lfedVersion,
		"LFED_CONTEXT=" + string(ctx.Name),
		"LFED_API_BASE_URL=" + ctx.APIBaseURL(),
		"LFED_TOKEN=" + token,
		"LFED_OUTPUT=" + string(mode),
		"LFED_REEXEC=" + reexec,
	}
}

// Run looks up name, checks requires_lfed and executes the plugin with
// args. It is the unknown-command path of the root command.
func Run(ctx context.Context, f *cmdutil.Factory, name string, args []string) error {
	p, err := Lookup(name)
	if err != nil {
		return err
	}
	if err := checkInstalled(p, f.Version); err != nil {
		return err
	}
	return Exec(ctx, f, p, args)
}

// checkInstalled runs CheckLfedVersion and fills the plugin name into the
// error.
func checkInstalled(p Installed, running string) error {
	err := CheckLfedVersion(p.Manifest.RequiresLfed, running)
	var incompatible *IncompatibleError
	if errors.As(err, &incompatible) {
		incompatible.Name = p.Manifest.Name
	}
	return err
}

// Exec runs the plugin binary with args and the contract environment.
// Streams pass through. --yes is appended when the CLI flag is set and
// args do not already carry it. A non-zero exit returns *ExitError.
//
// The token is best effort. When it cannot be resolved, LFED_TOKEN is
// exported empty and the plugin asks `LFED_REEXEC auth token` when it
// needs one (sdk/auth.Token). That keeps --help and read-only paths
// working while logged out; the login hint appears at first API use.
func Exec(ctx context.Context, f *cmdutil.Factory, p Installed, args []string) error {
	token, _ := f.Token(ctx)
	env, err := cliEnv(f, token)
	if err != nil {
		return err
	}
	if f.Yes && !slices.Contains(args, "--yes") {
		args = append(slices.Clone(args), "--yes")
	}
	return run(ctx, f, p.Binary, env, args)
}

// Complete runs `lfed-<name> __complete args...` for shell completion. The
// plugin's completion output passes through unchanged. No token is
// resolved: completion MUST NOT prompt for the keyring or fail on auth.
func Complete(ctx context.Context, f *cmdutil.Factory, name string, args []string) error {
	p, err := Lookup(name)
	if err != nil {
		return err
	}
	if err := checkInstalled(p, f.Version); err != nil {
		return err
	}
	env, err := cliEnv(f, "")
	if err != nil {
		return err
	}
	return run(ctx, f, p.Binary, env, append([]string{"__complete"}, args...))
}

// cliEnv resolves context, output mode and the lfed binary path, then
// appends Env to the process environment. Later entries win in os/exec,
// so LFED_* values set by the user are replaced.
func cliEnv(f *cmdutil.Factory, token string) ([]string, error) {
	c, err := f.Context()
	if err != nil {
		return nil, err
	}
	mode, err := f.Output()
	if err != nil {
		return nil, err
	}
	exe, err := f.Executable()
	if err != nil {
		return nil, fmt.Errorf("resolve lfed path: %w", err)
	}
	return append(os.Environ(), Env(f.Version, c, token, mode, exe)...), nil
}

func run(ctx context.Context, f *cmdutil.Factory, bin string, env, args []string) error {
	cmd := exec.CommandContext(ctx, bin, args...)
	cmd.Env = env
	cmd.Stdin = f.IOStreams.In
	cmd.Stdout = f.IOStreams.Out
	cmd.Stderr = f.IOStreams.ErrOut
	f.Debugf("exec %s %s", bin, strings.Join(args, " "))
	for _, kv := range env {
		if strings.HasPrefix(kv, "LFED_") && !strings.HasPrefix(kv, "LFED_TOKEN=") {
			f.Debugf("env %s", kv)
		} else if strings.HasPrefix(kv, "LFED_TOKEN=") {
			f.Debugf("env LFED_TOKEN=%s", redact(strings.TrimPrefix(kv, "LFED_TOKEN=")))
		}
	}
	err := cmd.Run()
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if code < 0 {
			// Killed by a signal: no exit code exists, report failure.
			code = 1
		}
		f.Debugf("exit %d", code)
		return &ExitError{Code: code}
	}
	f.Debugf("exit 0")
	if err != nil {
		return fmt.Errorf("run %s: %w", bin, err)
	}
	return nil
}

// redact shows whether a token is present without showing it.
func redact(token string) string {
	if token == "" {
		return "(empty)"
	}
	return fmt.Sprintf("(%d chars)", len(token))
}
