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

type ExitError struct {
	Code int
}

func (e *ExitError) Error() string {
	return fmt.Sprintf("plugin exited with code %d", e.Code)
}

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

func checkInstalled(p Installed, running string) error {
	err := CheckLfedVersion(p.Manifest.RequiresLfed, running)
	var incompatible *IncompatibleError
	if errors.As(err, &incompatible) {
		incompatible.Name = p.Manifest.Name
	}
	return err
}

func Exec(_ context.Context, f *cmdutil.Factory, p Installed, args []string) error {
	env, err := cliEnv(f)
	if err != nil {
		return err
	}
	return run(f, p.Binary, env, args)
}

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

const relaySignal = syscall.SIGTERM

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
