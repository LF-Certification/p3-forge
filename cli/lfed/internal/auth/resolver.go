package auth

import (
	"context"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/contexts"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/credstore"
)

// OpenStore resolves the active context and opens its credential store
// with the backend selected by --insecure-storage.
func OpenStore(f *cmdutil.Factory) (contexts.Context, credstore.Store, error) {
	c, err := f.Context()
	if err != nil {
		return contexts.Context{}, nil, err
	}
	store, err := credstore.New(credstore.Options{Context: string(c.Name), Insecure: f.InsecureStorage})
	if err != nil {
		return contexts.Context{}, nil, err
	}
	return c, store, nil
}

// TokenResolver returns the Factory.Token implementation. main installs it
// so every command and the plugin manager share one refresh path.
func TokenResolver(f *cmdutil.Factory) func(context.Context) (string, error) {
	return func(ctx context.Context) (string, error) {
		c, store, err := OpenStore(f)
		if err != nil {
			return "", err
		}
		return Token(ctx, c, store, f.InsecureStorage)
	}
}
