package pluginmgr

import (
	"context"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"os/signal"
	"strings"
	"syscall"

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
// code for *ExitError, 2 for *NotFoundError, *IncompatibleError,
// *NoReleaseError and *cmdutil.UsageError, else 1.
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
	var usage *cmdutil.UsageError
	if errors.As(err, &notFound) || errors.As(err, &incompatible) || errors.As(err, &noRelease) || errors.As(err, &usage) {
		return 2
	}
	return 1
}

// Env returns the LFED_* variables of the plugin contract. The base URL
// has no trailing slash; contexts.Context.APIBaseURL guarantees that.
// LFED_YES and LFED_INSECURE_STORAGE are "1" when the flag is set and
// empty otherwise, so a value the operator exported cannot pass through.
// LFED_DEBUG is not set here: it passes through from the operator's
// environment, so a plugin sees exactly what the CLI saw.
func Env(lfedVersion string, ctx contexts.Context, mode output.Mode, reexec string, yes, insecureStorage bool) []string {
	return []string{
		"LFED_CLI_VERSION=" + lfedVersion,
		"LFED_CONTEXT=" + string(ctx.Name),
		"LFED_API_BASE_URL=" + ctx.APIBaseURL(),
		"LFED_OUTPUT=" + string(mode),
		"LFED_REEXEC=" + reexec,
		"LFED_YES=" + flagValue(yes),
		"LFED_INSECURE_STORAGE=" + flagValue(insecureStorage),
	}
}

func flagValue(set bool) string {
	if set {
		return "1"
	}
	return ""
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
// Streams and args pass through unchanged. A non-zero exit returns
// *ExitError. ctx is not used to stop the plugin; see run.
//
// No token is exported. The plugin asks `LFED_REEXEC auth token` when it
// needs one (sdk/auth.Token). That keeps --help and read-only paths
// working while logged out; the login hint appears at first API use.
func Exec(_ context.Context, f *cmdutil.Factory, p Installed, args []string) error {
	env, err := cliEnv(f)
	if err != nil {
		return err
	}
	return run(f, p.Binary, env, args)
}

// Complete runs `lfed-<name> __complete args...` for shell completion. The
// plugin's completion output passes through unchanged. It gets the same
// environment as Exec.
func Complete(_ context.Context, f *cmdutil.Factory, name string, args []string) error {
	p, err := Lookup(name)
	if err != nil {
		return err
	}
	if err := checkInstalled(p, f.Version); err != nil {
		return err
	}
	env, err := cliEnv(f)
	if err != nil {
		return err
	}
	return run(f, p.Binary, env, append([]string{"__complete"}, args...))
}

// cliEnv resolves context, output mode and the lfed binary path, and
// returns Env for them.
func cliEnv(f *cmdutil.Factory) ([]string, error) {
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
	return Env(f.Version, c, mode, exe, f.Yes, f.InsecureStorage), nil
}

// relaySignal is the signal run relays to the plugin. SIGINT is not
// relayed: a terminal delivers it to the whole foreground process group,
// so the plugin already has it. A second SIGINT would make tools such as
// terraform force-quit.
const relaySignal = syscall.SIGTERM

// run executes bin with the process environment plus contract. Later
// entries win in os/exec, so LFED_* values set by the operator are
// replaced. The plugin owns its lifetime: run waits for it to exit and
// never kills it. A relaySignal received while it runs is relayed. A
// plugin killed by signal N returns *ExitError with code 128+N.
func run(f *cmdutil.Factory, bin string, contract, args []string) error {
	cmd := exec.Command(bin, args...)
	cmd.Env = append(os.Environ(), contract...)
	cmd.Stdin = f.IOStreams.In
	cmd.Stdout = f.IOStreams.Out
	cmd.Stderr = f.IOStreams.ErrOut
	f.Debugf("exec %s %s", bin, strings.Join(args, " "))
	for _, kv := range contract {
		f.Debugf("env %s", kv)
	}
	// Notify before Start: a signal that arrives in between is buffered
	// and relayed once the process exists.
	relay := make(chan os.Signal, 1)
	signal.Notify(relay, relaySignal)
	if err := cmd.Start(); err != nil {
		signal.Stop(relay)
		return fmt.Errorf("run %s: %w", bin, err)
	}
	stop := make(chan struct{})
	go func() {
		for {
			select {
			case sig := <-relay:
				_ = cmd.Process.Signal(sig)
			case <-stop:
				return
			}
		}
	}()
	err := cmd.Wait()
	signal.Stop(relay)
	close(stop)
	var exit *exec.ExitError
	if errors.As(err, &exit) {
		code := exit.ExitCode()
		if code < 0 {
			code = signalExitCode(exit)
		}
		f.Debugf("exit %d", code)
		return &ExitError{Code: code}
	}
	if err != nil {
		return fmt.Errorf("run %s: %w", bin, err)
	}
	f.Debugf("exit 0")
	return nil
}
