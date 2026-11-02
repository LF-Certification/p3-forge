// Package auth implements `lfed auth login|logout|status|token`.
package auth

import (
	"bufio"
	"errors"
	"fmt"
	"io"
	"time"

	"github.com/spf13/cobra"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/auth"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/cmdutil"
	"github.com/LF-Certification/p3-forge/cli/lfed/sdk/output"
)

// NewCmd builds the `lfed auth` group.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "auth",
		Short: "Log in to the education platform and manage credentials",
	}
	cmd.AddCommand(newLoginCmd(f), newLogoutCmd(f), newStatusCmd(f), newTokenCmd(f))
	return cmd
}

func newLoginCmd(f *cmdutil.Factory) *cobra.Command {
	var withToken, noBrowser bool
	cmd := &cobra.Command{
		Use:   "login",
		Short: "Log in with the Auth0 device code flow",
		Long: `Log in to the active context with the Auth0 device code flow.

The verification URL opens in the default browser. Pass --no-browser to
print the URL only. Pass --with-token to read a refresh token from stdin
instead, for headless use:

  echo "$REFRESH_TOKEN" | lfed auth login --with-token`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			c, store, err := auth.OpenStore(f)
			if err != nil {
				return err
			}
			errOut := f.IOStreams.ErrOut
			if withToken {
				line, err := bufio.NewReader(f.IOStreams.In).ReadString('\n')
				if err != nil && !errors.Is(err, io.EOF) {
					return fmt.Errorf("read refresh token from stdin: %w", err)
				}
				if err := auth.TokenLogin(store, line, f.InsecureStorage); err != nil {
					return err
				}
				fmt.Fprintf(errOut, "Logged in to %s with a supplied refresh token.\n", c.Name)
				return nil
			}
			identity, err := auth.DeviceLogin(cmd.Context(), c, store, auth.LoginOptions{
				Out:         errOut,
				OpenBrowser: !noBrowser,
				Insecure:    f.InsecureStorage,
			})
			if err != nil {
				return err
			}
			if identity != "" {
				fmt.Fprintf(errOut, "Logged in to %s as %s.\n", c.Name, identity)
			} else {
				fmt.Fprintf(errOut, "Logged in to %s.\n", c.Name)
			}
			return nil
		},
	}
	cmd.Flags().BoolVar(&withToken, "with-token", false, "Read a refresh token from stdin instead of using the browser")
	cmd.Flags().BoolVar(&noBrowser, "no-browser", false, "Print the verification URL instead of opening a browser")
	return cmd
}

func newLogoutCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "logout",
		Short: "Remove the stored credentials for the active context",
		Args:  cobra.NoArgs,
		RunE: func(_ *cobra.Command, _ []string) error {
			c, store, err := auth.OpenStore(f)
			if err != nil {
				return err
			}
			if err := auth.Logout(store, f.InsecureStorage); err != nil {
				return err
			}
			fmt.Fprintf(f.IOStreams.ErrOut, "Logged out of %s.\n", c.Name)
			return nil
		},
	}
}

// statusRow is the `auth status` record.
type statusRow struct {
	Context  string    `json:"context"`
	Username string    `json:"username"`
	Issuer   string    `json:"issuer"`
	Expires  time.Time `json:"expires"`
}

func newStatusCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "status",
		Short: "Show the login for the active context",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			mode, err := f.Output()
			if err != nil {
				return err
			}
			c, err := f.Context()
			if err != nil {
				return err
			}
			token, err := f.Token(cmd.Context())
			if err != nil {
				return err
			}
			claims, err := auth.DecodeClaims(token)
			if err != nil {
				return err
			}
			row := statusRow{
				Context:  string(c.Name),
				Username: claims.Username,
				Issuer:   claims.Issuer,
				Expires:  claims.Expiry,
			}
			return output.PrintOne(f.IOStreams.Out, mode, row, []output.Column[statusRow]{
				{Header: "Context", Value: func(r statusRow) string { return r.Context }},
				{Header: "Username", Value: func(r statusRow) string { return r.Username }},
				{Header: "Issuer", Value: func(r statusRow) string { return r.Issuer }},
				{Header: "Expires", Value: func(r statusRow) string { return r.Expires.Local().Format(time.RFC3339) }},
			})
		},
	}
}

func newTokenCmd(f *cmdutil.Factory) *cobra.Command {
	return &cobra.Command{
		Use:   "token",
		Short: "Print a valid access token for the active context",
		Long: `Print a valid access token for the active context.

The token is refreshed when it expires within 60 seconds. Plugins call
this through LFED_REEXEC when LFED_TOKEN is empty.`,
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			token, err := f.Token(cmd.Context())
			if err != nil {
				return err
			}
			_, err = fmt.Fprintln(f.IOStreams.Out, token)
			return err
		},
	}
}
