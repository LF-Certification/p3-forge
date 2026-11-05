// Package auth implements the Auth0 device code login and token refresh
// for one context. Commands in internal/cmd/auth are thin wrappers.
package auth

import (
	"context"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os/exec"
	"runtime"
	"strconv"
	"strings"
	"time"
	"unicode"

	"golang.org/x/oauth2"

	"github.com/LF-Certification/p3-forge/cli/lfed/internal/contexts"
	"github.com/LF-Certification/p3-forge/cli/lfed/internal/credstore"
)

// Scopes requested at login. offline_access yields the refresh token; the
// others identify the user.
var Scopes = []string{"openid", "email", "profile", "offline_access"}

// Auth0BaseURL returns the tenant base URL for c without a trailing slash.
// Tests replace it with an httptest server URL.
var Auth0BaseURL = func(c contexts.Context) string {
	return "https://" + c.Auth0Domain
}

// ErrNotLoggedIn is returned when no credentials exist for the context.
var ErrNotLoggedIn = errors.New(`not logged in; run "lfed auth login"`)

// ErrSessionExpired is returned when Auth0 rejects the refresh token.
var ErrSessionExpired = errors.New(`session expired or revoked; run "lfed auth login"`)

// ErrNoRefreshToken is returned when the cached token is stale and no
// refresh token exists to renew it.
var ErrNoRefreshToken = errors.New(`no refresh token stored; run "lfed auth login"`)

// httpTimeout bounds one request to Auth0.
const httpTimeout = 30 * time.Second

// withHTTPClient makes oauth2 use a client with httpTimeout for every
// request made with the returned ctx. The device code poll loop still ends
// only when ctx ends or the device code expires.
func withHTTPClient(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: httpTimeout})
}

// oauthConfig builds the oauth2 client for c. Auth0 reads client_id from
// the form body, so AuthStyleInParams is required.
func oauthConfig(c contexts.Context) *oauth2.Config {
	base := Auth0BaseURL(c)
	return &oauth2.Config{
		ClientID: c.ClientID,
		Scopes:   Scopes,
		Endpoint: oauth2.Endpoint{
			DeviceAuthURL: base + "/oauth/device/code",
			TokenURL:      base + "/oauth/token",
			AuthStyle:     oauth2.AuthStyleInParams,
		},
	}
}

// CheckBackend compares the stored login state with the --insecure-storage
// flag. The state file is shared by both backends, so a mismatch means the
// credentials live in the other backend. A missing state is ErrNotLoggedIn:
// the state file is written at login and removed at logout, so callers
// MUST NOT touch the system store before this check. A locked Secret
// Service collection prompts on first access; a logged-out user must not
// see that prompt.
func CheckBackend(store credstore.Store, insecure bool) error {
	state, err := store.LoadDeviceState()
	if errors.Is(err, credstore.ErrNotFound) {
		return ErrNotLoggedIn
	}
	if err != nil {
		return fmt.Errorf("load login state: %w", err)
	}
	switch {
	case state.Insecure && !insecure:
		return errors.New("credentials were stored with --insecure-storage; pass --insecure-storage")
	case !state.Insecure && insecure:
		return errors.New("credentials are in the system keychain; omit --insecure-storage")
	}
	return nil
}

// LoginOptions controls the interactive device code flow.
type LoginOptions struct {
	// Out receives the user code and verification URL. Use stderr so
	// `lfed auth login` never pollutes a piped stdout.
	Out io.Writer
	// OpenBrowser launches the verification URL in the default browser.
	OpenBrowser bool
	// Insecure selects the backend recorded in the login state; see
	// CheckBackend.
	Insecure bool
}

// DeviceLogin runs the device code flow for c and persists the result in
// store. It returns the identity from the id_token, or "" when absent.
func DeviceLogin(ctx context.Context, c contexts.Context, store credstore.Store, opts LoginOptions) (string, error) {
	cfg := oauthConfig(c)
	httpCtx := withHTTPClient(ctx)
	resp, err := cfg.DeviceAuth(httpCtx, oauth2.SetAuthURLParam("audience", c.Audience))
	if err != nil {
		return "", fmt.Errorf("request device code: %w", err)
	}
	if err := printDeviceCodeInstructions(opts.Out, resp, opts.OpenBrowser); err != nil {
		return "", fmt.Errorf("display login instructions: %w", err)
	}
	fmt.Fprintln(opts.Out, "Waiting for authentication...")

	token, err := cfg.DeviceAccessToken(httpCtx, resp)
	if err != nil {
		// oauth2 stops at the device code expiry with DeadlineExceeded
		// before the server can answer expired_token. Both mean the
		// verification window ran out.
		var retrieveErr *oauth2.RetrieveError
		switch {
		case errors.Is(err, context.DeadlineExceeded):
			return "", errors.New("timed out waiting for login")
		case errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "expired_token":
			return "", errors.New("timed out waiting for login")
		case errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "access_denied":
			return "", errors.New("login was denied")
		default:
			return "", fmt.Errorf("wait for login: %w", err)
		}
	}
	// Auth0 can omit the refresh token when offline access is disabled
	// for the audience. A login without one stops working at expiry, so
	// fail now.
	if token.RefreshToken == "" {
		return "", errors.New("login did not return a refresh token; offline access may be disabled for this API")
	}

	if err := persistLogin(ctx, store, credstore.Credentials{
		RefreshToken:      token.RefreshToken,
		AccessToken:       token.AccessToken,
		AccessTokenExpiry: token.Expiry,
	}, opts.Insecure); err != nil {
		return "", err
	}
	// The id_token is not verified: lfed only displays its identity, and
	// it arrives over the TLS-authenticated token endpoint.
	idToken, _ := token.Extra("id_token").(string)
	return IdentityFromIDToken(idToken), nil
}

// TokenLogin stores a refresh token supplied by the user. No access token
// is cached; the next Token call exchanges the refresh token for one.
func TokenLogin(ctx context.Context, store credstore.Store, refreshToken string, insecure bool) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return errors.New("no refresh token provided on stdin")
	}
	return persistLogin(ctx, store, credstore.Credentials{RefreshToken: refreshToken}, insecure)
}

// persistLogin saves credentials and login state as a pair under the
// context lock. When the state write fails, the new credentials are
// deleted so no login exists without a matching backend record. When the
// previous login used the other backend, its secret is deleted after the
// state write, so a backend switch leaves no old refresh token behind.
func persistLogin(ctx context.Context, store credstore.Store, creds credstore.Credentials, insecure bool) error {
	unlock, err := store.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	prev, err := store.LoadDeviceState()
	switch {
	case errors.Is(err, credstore.ErrNotFound):
		// The previous backend is unknown. Do not touch the other
		// backend; see CheckBackend for the Secret Service prompt.
		prev.Insecure = insecure
	case err != nil:
		return fmt.Errorf("load login state: %w", err)
	}
	if err := store.SaveCredentials(creds); err != nil {
		return fmt.Errorf("save credentials: %w", err)
	}
	if err := store.SaveDeviceState(credstore.DeviceState{Insecure: insecure}); err != nil {
		if delErr := store.DeleteCredentials(insecure); delErr != nil {
			return fmt.Errorf("save login state: %w (rollback of credentials also failed: %v)", err, delErr)
		}
		return fmt.Errorf("save login state: %w", err)
	}
	if prev.Insecure != insecure {
		if err := store.DeleteCredentials(prev.Insecure); err != nil {
			return fmt.Errorf("remove previous credentials: %w", err)
		}
	}
	return nil
}

// Logout removes credentials and login state for the store's context,
// whatever backend --insecure-storage selects. The plain file is always
// deleted. Without a login state, the system store is not touched: a
// locked Secret Service collection prompts on first access, and a
// logged-out user must not see that prompt. With a login state, the
// secret is deleted from both backends. A failure in the backend that the
// state records is an error. A failure in the other backend is returned
// as a warning: on a headless host without a system store, the cleanup
// of the system store cannot work.
func Logout(ctx context.Context, store credstore.Store) (warning string, err error) {
	unlock, err := store.Lock(ctx)
	if err != nil {
		return "", err
	}
	defer unlock()
	state, err := store.LoadDeviceState()
	if errors.Is(err, credstore.ErrNotFound) {
		if err := store.DeleteCredentials(true); err != nil {
			return "", fmt.Errorf("delete credentials: %w", err)
		}
		return "", nil
	}
	if err != nil {
		return "", fmt.Errorf("load login state: %w", err)
	}
	if err := store.DeleteCredentials(state.Insecure); err != nil {
		return "", fmt.Errorf("delete credentials: %w", err)
	}
	if err := store.DeleteCredentials(!state.Insecure); err != nil {
		warning = fmt.Sprintf("could not remove old credentials from the other backend: %v", err)
	}
	if err := store.DeleteDeviceState(); err != nil {
		return warning, fmt.Errorf("delete login state: %w", err)
	}
	return warning, nil
}

// Token returns a valid access token for c. It returns the cached token
// when it expires more than 60 seconds from now. Otherwise it refreshes
// and persists the new credentials. The context lock is held from load to
// save, so concurrent lfed processes redeem a refresh token only once.
func Token(ctx context.Context, c contexts.Context, store credstore.Store, insecure bool) (string, error) {
	if err := CheckBackend(store, insecure); err != nil {
		return "", err
	}
	unlock, err := store.Lock(ctx)
	if err != nil {
		return "", err
	}
	defer unlock()
	creds, err := store.LoadCredentials()
	if errors.Is(err, credstore.ErrNotFound) {
		return "", ErrNotLoggedIn
	}
	if err != nil {
		return "", err
	}
	if creds.ValidAccessToken() {
		return creds.AccessToken, nil
	}
	if creds.RefreshToken == "" {
		return "", ErrNoRefreshToken
	}

	refreshed, err := oauthConfig(c).TokenSource(withHTTPClient(ctx), &oauth2.Token{RefreshToken: creds.RefreshToken}).Token()
	var retrieveErr *oauth2.RetrieveError
	if errors.As(err, &retrieveErr) && retrieveErr.ErrorCode == "invalid_grant" {
		return "", ErrSessionExpired
	}
	if err != nil {
		return "", fmt.Errorf("refresh access token: %w", err)
	}

	// Auth0 rotates the refresh token only when rotation is enabled on the
	// client. Keep the old one otherwise.
	refreshToken := refreshed.RefreshToken
	if refreshToken == "" {
		refreshToken = creds.RefreshToken
	}
	if err := store.SaveCredentials(credstore.Credentials{
		RefreshToken:      refreshToken,
		AccessToken:       refreshed.AccessToken,
		AccessTokenExpiry: refreshed.Expiry,
	}); err != nil {
		return "", fmt.Errorf("save refreshed credentials: %w", err)
	}
	return refreshed.AccessToken, nil
}

// printDeviceCodeInstructions shows the IdP-supplied code and URL safely.
// The manual instructions remain when the browser does not open.
func printDeviceCodeInstructions(w io.Writer, resp *oauth2.DeviceAuthResponse, openInBrowser bool) error {
	if _, err := fmt.Fprintf(w, "First copy your one-time code: %s\n", terminalSafeText(resp.UserCode)); err != nil {
		return err
	}
	// VerificationURIComplete is optional per RFC 8628 section 3.2.
	verificationURI := resp.VerificationURIComplete
	if verificationURI == "" {
		verificationURI = resp.VerificationURI
	}
	displayURI := terminalSafeText(verificationURI)
	if !openInBrowser {
		_, err := fmt.Fprintf(w, "Then visit: %s\n", displayURI)
		return err
	}
	if _, err := fmt.Fprintf(w, "Opening %s in your browser...\n", displayURI); err != nil {
		return err
	}
	if err := openBrowser(verificationURI); err != nil {
		if _, writeErr := fmt.Fprintf(w, "Could not open the browser: %v\n", err); writeErr != nil {
			return writeErr
		}
		_, writeErr := fmt.Fprintf(w, "Please visit: %s\n", displayURI)
		return writeErr
	}
	return nil
}

// terminalSafeText quotes strings that contain control characters so an
// IdP value cannot emit terminal escape sequences or forge output lines.
func terminalSafeText(s string) string {
	if strings.ContainsFunc(s, unicode.IsControl) {
		return strconv.QuoteToASCII(s)
	}
	return s
}

// validateBrowserURL accepts only absolute HTTPS URLs. OS launchers also
// open local files and custom schemes, so exec alone is not safe.
func validateBrowserURL(uri string) error {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.Opaque != "" || strings.ContainsFunc(uri, unicode.IsControl) {
		return errors.New("refusing to open verification URI: expected an absolute HTTPS URL with a hostname")
	}
	return nil
}

// openBrowser opens a validated HTTPS URL with the per-OS launcher.
func openBrowser(uri string) error {
	if err := validateBrowserURL(uri); err != nil {
		return err
	}
	switch runtime.GOOS {
	case "darwin":
		return exec.Command("open", uri).Start()
	case "windows":
		return exec.Command("rundll32", "url.dll,FileProtocolHandler", uri).Start()
	default:
		return exec.Command("xdg-open", uri).Start()
	}
}
