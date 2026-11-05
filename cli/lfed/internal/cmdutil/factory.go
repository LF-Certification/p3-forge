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

type IOStreams struct {
	In     io.Reader
	Out    io.Writer
	ErrOut io.Writer
}

type Factory struct {
	IOStreams IOStreams

	Version string

	ContextFlag     string
	OutputFlag      string
	JQ              string
	Yes             bool
	InsecureStorage bool

	Debug bool

	Capture *OutputCapture

	Config func() (*config.Config, error)

	Context func() (contexts.Context, error)

	Token func(ctx context.Context) (string, error)

	Executable func() (string, error)
}

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

func (f *Factory) Debugf(format string, args ...any) {
	if f.Debug {
		fmt.Fprintf(f.IOStreams.ErrOut, "lfed: debug: "+format+"\n", args...)
	}
}

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

func (c *OutputCapture) Start() {
	if c.buf == nil {
		c.buf = &bytes.Buffer{}
	}
}

func (c *OutputCapture) Stop() (data []byte, ok bool) {
	if c.buf == nil {
		return nil, false
	}
	data = c.buf.Bytes()
	c.buf = nil
	return data, true
}
