// Package cmdutil holds the Factory injected into every command package.
package cmdutil

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"os"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/config"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/contexts"
	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

// IOStreams are the process streams. Tests replace them with buffers.
type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

// Factory carries shared state and lazily resolved dependencies. Command
// constructors take *Factory and read it at run time, not at build time,
// so persistent flags parsed by Cobra are visible.
type Factory struct {
	IOStreams IOStreams
	// Version is the lfed CLI semver (internal/build.Version).
	Version string

	// Root persistent flags. Cobra writes them before any RunE runs.
	ContextFlag     string // -c/--context
	OutputFlag      string // -o/--output
	JQ              string // --jq
	Yes             bool   // --yes
	InsecureStorage bool   // --insecure-storage

	// Debug is true when LFED_DEBUG is set. Debugf writes to ErrOut only then.
	Debug bool
	// Capture, when set, is the IOStreams.Out writer. --jq starts a
	// capture so the JSON document can be filtered after the command ran.
	Capture *OutputCapture

	// Config loads $XDG_CONFIG_HOME/lfed/config.yaml.
	Config func() (*config.Config, error)
	// Context resolves the active context: -c, LFED_CONTEXT, config file.
	Context func() (contexts.Context, error)
	// Token returns a valid access token for the active context. It
	// refreshes when the cached token expires within 60 seconds. The auth
	// package installs it in main.
	Token func(ctx context.Context) (string, error)
	// Executable returns the absolute path of the running lfed binary. The
	// plugin manager exports it as LFED_REEXEC.
	Executable func() (string, error)
}

// New builds a Factory bound to the process streams.
func New(version string) *Factory {
	f := &Factory{
		IOStreams: IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr},
		Version:   version,
		Debug:     os.Getenv("LFED_DEBUG") != "",
		Config:    config.Load,
		Executable: func() (string, error) {
			return os.Executable()
		},
	}
	f.Context = func() (contexts.Context, error) {
		cfg, err := f.Config()
		if err != nil {
			return contexts.Context{}, err
		}
		return contexts.Resolve(f.ContextFlag, os.Getenv("LFED_CONTEXT"), cfg.CurrentContext)
	}
	return f
}

// Output returns the validated -o mode. --jq implies json: the filter
// runs over the JSON document, so a table or yaml request alongside it is
// an error rather than a silent override.
func (f *Factory) Output() (output.Mode, error) {
	mode, err := output.ParseMode(f.OutputFlag)
	if err != nil || f.JQ == "" {
		return mode, err
	}
	if mode != output.JSON && f.OutputFlag != "" && f.OutputFlag != string(output.Table) {
		return "", fmt.Errorf("--jq needs JSON output; drop -o %s", f.OutputFlag)
	}
	return output.JSON, nil
}

// Debugf writes one diagnostic line to ErrOut when LFED_DEBUG is set.
func (f *Factory) Debugf(format string, args ...any) {
	if f.Debug {
		fmt.Fprintf(f.IOStreams.ErrOut, "lfed: debug: "+format+"\n", args...)
	}
}

// OutputCapture is a writer that passes through to Real until Start, then
// buffers until Stop. It lets --jq post-process stdout without the
// commands or plugins knowing.
type OutputCapture struct {
	Real io.Writer
	buf  *bytes.Buffer
}

func (c *OutputCapture) Write(p []byte) (int, error) {
	if c.buf != nil {
		return c.buf.Write(p)
	}
	return c.Real.Write(p)
}

// Start begins buffering. Calling it twice keeps the first buffer.
func (c *OutputCapture) Start() {
	if c.buf == nil {
		c.buf = &bytes.Buffer{}
	}
}

// Stop ends buffering and returns what was captured; ok is false when
// Start was never called.
func (c *OutputCapture) Stop() (data []byte, ok bool) {
	if c.buf == nil {
		return nil, false
	}
	data = c.buf.Bytes()
	c.buf = nil
	return data, true
}
