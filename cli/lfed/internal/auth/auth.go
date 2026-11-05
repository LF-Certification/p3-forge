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

var Scopes = []string{"openid", "email", "profile", "offline_access"}

var Auth0BaseURL = func(c contexts.Context) string {
	return "https://" + c.Auth0Domain
}

var ErrNotLoggedIn = errors.New(`not logged in; run "lfed auth login"`)

var ErrSessionExpired = errors.New(`session expired or revoked; run "lfed auth login"`)

var ErrNoRefreshToken = errors.New(`no refresh token stored; run "lfed auth login"`)

const httpTimeout = 30 * time.Second

func withHTTPClient(ctx context.Context) context.Context {
	return context.WithValue(ctx, oauth2.HTTPClient, &http.Client{Timeout: httpTimeout})
}

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

type LoginOptions struct {
	Out io.Writer

	OpenBrowser bool

	Insecure bool
}

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

	idToken, _ := token.Extra("id_token").(string)
	return IdentityFromIDToken(idToken), nil
}

func TokenLogin(ctx context.Context, store credstore.Store, refreshToken string, insecure bool) error {
	refreshToken = strings.TrimSpace(refreshToken)
	if refreshToken == "" {
		return errors.New("no refresh token provided on stdin")
	}
	return persistLogin(ctx, store, credstore.Credentials{RefreshToken: refreshToken}, insecure)
}

func persistLogin(ctx context.Context, store credstore.Store, creds credstore.Credentials, insecure bool) error {
	unlock, err := store.Lock(ctx)
	if err != nil {
		return err
	}
	defer unlock()
	prev, err := store.LoadDeviceState()
	switch {
	case errors.Is(err, credstore.ErrNotFound):

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

func printDeviceCodeInstructions(w io.Writer, resp *oauth2.DeviceAuthResponse, openInBrowser bool) error {
	if _, err := fmt.Fprintf(w, "First copy your one-time code: %s\n", terminalSafeText(resp.UserCode)); err != nil {
		return err
	}

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

func terminalSafeText(s string) string {
	if strings.ContainsFunc(s, unicode.IsControl) {
		return strconv.QuoteToASCII(s)
	}
	return s
}

func validateBrowserURL(uri string) error {
	u, err := url.Parse(uri)
	if err != nil || u.Scheme != "https" || u.Hostname() == "" || u.Opaque != "" || strings.ContainsFunc(uri, unicode.IsControl) {
		return errors.New("refusing to open verification URI: expected an absolute HTTPS URL with a hostname")
	}
	return nil
}

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
