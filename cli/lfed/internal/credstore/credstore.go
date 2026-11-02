// Package credstore stores lfed credentials and login state per context.
//
// Secrets (refresh token, cached access token, expiry) go to the system
// credential store through github.com/zalando/go-keyring: Secret Service
// on Linux (the session default collection, unlocked at login), Keychain
// on macOS, Credential Manager on Windows. There is no file fallback: a
// store that cannot be reached is an error at first use.
//
// With Options.Insecure the secrets go to a plain owner-only JSON file.
// Use this for headless or CI runs where no passphrase prompt is possible.
//
// Non-secret login state goes to a plain JSON file in the state directory.
// Every item is keyed by context name, so a login on staging never touches
// prod credentials. TestContextIsolation checks this.
package credstore

import (
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"
	"time"

	"github.com/zalando/go-keyring"
)

// ErrNotFound is returned by Load methods when nothing is stored yet.
var ErrNotFound = errors.New("credstore: not found")

// serviceName identifies lfed items in shared credential stores.
const serviceName = "lfed"

// Credentials holds the secrets for one context: the long-lived Auth0
// refresh token and an optional cached access token with its expiry.
type Credentials struct {
	RefreshToken      string    `json:"refresh_token"`
	AccessToken       string    `json:"access_token,omitempty"`
	AccessTokenExpiry time.Time `json:"access_token_expiry,omitempty"`
}

// refreshLeeway is the window before expiry in which a cached token counts
// as expired. The plugin contract requires a refresh inside 60 seconds.
const refreshLeeway = 60 * time.Second

// ValidAccessToken reports whether the cached access token is present and
// expires more than 60 seconds from now.
func (c Credentials) ValidAccessToken() bool {
	return c.AccessToken != "" && time.Now().Add(refreshLeeway).Before(c.AccessTokenExpiry)
}

// DeviceState is the non-secret record of the last login for one context.
type DeviceState struct {
	// Insecure records whether the login used the plain-file backend.
	// The state file is shared by both backends, so callers compare this
	// with the --insecure-storage flag before they trust the credentials.
	Insecure bool `json:"insecure,omitempty"`
}

// Store is the credential storage abstraction used by the auth commands.
type Store interface {
	// SaveCredentials persists the secrets for the configured context.
	SaveCredentials(creds Credentials) error
	// LoadCredentials returns the persisted secrets, or ErrNotFound.
	LoadCredentials() (Credentials, error)
	// DeleteCredentials removes the persisted secrets. Missing is a no-op.
	DeleteCredentials() error

	// SaveDeviceState persists non-secret login state as plain JSON.
	SaveDeviceState(state DeviceState) error
	// LoadDeviceState returns the persisted login state, or ErrNotFound.
	LoadDeviceState() (DeviceState, error)
	// DeleteDeviceState removes the persisted login state. Missing is a no-op.
	DeleteDeviceState() error
}

// Options configures a Store returned by New.
type Options struct {
	// Context is the context name (dev, staging, prod). Required.
	Context string
	// Insecure selects the plain-file backend instead of the system store.
	// It corresponds to the --insecure-storage flag.
	Insecure bool
	// StateDir overrides the state directory. Leave empty outside tests.
	StateDir string
}

// secretsBackend is one way to persist Credentials.
type secretsBackend interface {
	Save(creds Credentials) error
	Load() (Credentials, error)
	Delete() error
}

type store struct {
	secrets   secretsBackend
	statePath string
}

// New builds a Store for one context.
func New(opts Options) (Store, error) {
	if opts.Context == "" {
		return nil, errors.New("credstore: context is required")
	}
	stateDir, err := resolveStateDir(opts.StateDir)
	if err != nil {
		return nil, fmt.Errorf("credstore: resolve state dir: %w", err)
	}

	var secrets secretsBackend
	if opts.Insecure {
		secrets = &plainFileSecrets{
			path: filepath.Join(stateDir, "credentials-"+opts.Context+".json"),
		}
	} else {
		secrets = &keyringSecrets{user: "credentials/" + opts.Context}
	}

	return &store{
		secrets:   secrets,
		statePath: filepath.Join(stateDir, "state-"+opts.Context+".json"),
	}, nil
}

// resolveStateDir returns the state directory and creates it. The order is
// override, LFED_STATE_DIR, $XDG_STATE_HOME/lfed, ~/.local/state/lfed.
func resolveStateDir(override string) (string, error) {
	dir := override
	if dir == "" {
		dir = os.Getenv("LFED_STATE_DIR")
	}
	if dir == "" {
		// The XDG spec requires an absolute $XDG_STATE_HOME; ignore
		// relative values and use the default.
		if xdgState := os.Getenv("XDG_STATE_HOME"); filepath.IsAbs(xdgState) {
			dir = filepath.Join(xdgState, "lfed")
		} else {
			home, err := os.UserHomeDir()
			if err != nil {
				return "", fmt.Errorf("determine home directory: %w", err)
			}
			dir = filepath.Join(home, ".local", "state", "lfed")
		}
	}
	if err := os.MkdirAll(dir, 0o700); err != nil {
		return "", fmt.Errorf("create state directory %q: %w", dir, err)
	}
	return dir, nil
}

// writeOwnerOnlyFile writes data to path in place and creates it with mode
// 0600. The file has one writer, so no temp-file rename is needed. On
// Windows, 0600 maps to the read-only attribute and not to an ACL.
func writeOwnerOnlyFile(path string, data []byte) (err error) {
	f, err := os.OpenFile(path, os.O_WRONLY|os.O_CREATE|os.O_TRUNC, 0o600)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := f.Close(); err == nil {
			err = closeErr
		}
	}()
	if _, err = f.Write(data); err != nil {
		return err
	}
	return f.Sync()
}

func (s *store) SaveCredentials(creds Credentials) error { return s.secrets.Save(creds) }

func (s *store) LoadCredentials() (Credentials, error) { return s.secrets.Load() }

func (s *store) DeleteCredentials() error { return s.secrets.Delete() }

func (s *store) SaveDeviceState(state DeviceState) error {
	data, err := json.MarshalIndent(state, "", "  ")
	if err != nil {
		return fmt.Errorf("credstore: marshal device state: %w", err)
	}
	if err := writeOwnerOnlyFile(s.statePath, data); err != nil {
		return fmt.Errorf("credstore: write device state: %w", err)
	}
	return nil
}

func (s *store) LoadDeviceState() (DeviceState, error) {
	data, err := os.ReadFile(s.statePath)
	if errors.Is(err, os.ErrNotExist) {
		return DeviceState{}, ErrNotFound
	}
	if err != nil {
		return DeviceState{}, fmt.Errorf("credstore: read device state: %w", err)
	}
	var state DeviceState
	if err := json.Unmarshal(data, &state); err != nil {
		return DeviceState{}, fmt.Errorf("credstore: unmarshal device state: %w", err)
	}
	return state, nil
}

func (s *store) DeleteDeviceState() error {
	err := os.Remove(s.statePath)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("credstore: delete device state: %w", err)
	}
	return nil
}

// keyringSecrets stores Credentials in the system credential store as one
// JSON blob under (serviceName, user). Windows Credential Manager caps a
// blob at 2560 bytes; go-keyring reports that as ErrSetDataTooBig.
type keyringSecrets struct {
	user string
}

func (k *keyringSecrets) Save(creds Credentials) error {
	data, err := json.Marshal(creds)
	if err != nil {
		return fmt.Errorf("credstore: marshal credentials: %w", err)
	}
	if err := keyring.Set(serviceName, k.user, string(data)); err != nil {
		return fmt.Errorf("credstore: save credentials (use --insecure-storage as a fallback): %w", err)
	}
	return nil
}

func (k *keyringSecrets) Load() (Credentials, error) {
	data, err := keyring.Get(serviceName, k.user)
	if errors.Is(err, keyring.ErrNotFound) {
		return Credentials{}, ErrNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("credstore: load credentials (use --insecure-storage as a fallback): %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal([]byte(data), &creds); err != nil {
		return Credentials{}, fmt.Errorf("credstore: unmarshal credentials: %w", err)
	}
	return creds, nil
}

func (k *keyringSecrets) Delete() error {
	err := keyring.Delete(serviceName, k.user)
	if err != nil && !errors.Is(err, keyring.ErrNotFound) {
		return fmt.Errorf("credstore: delete credentials: %w", err)
	}
	return nil
}

// plainFileSecrets stores Credentials as an owner-only JSON file. It is
// less secure than keyringSecrets and is selected only by Options.Insecure.
type plainFileSecrets struct {
	path string
}

func (p *plainFileSecrets) Save(creds Credentials) error {
	data, err := json.MarshalIndent(creds, "", "  ")
	if err != nil {
		return fmt.Errorf("credstore: marshal credentials: %w", err)
	}
	if err := writeOwnerOnlyFile(p.path, data); err != nil {
		return fmt.Errorf("credstore: write credentials: %w", err)
	}
	return nil
}

func (p *plainFileSecrets) Load() (Credentials, error) {
	data, err := os.ReadFile(p.path)
	if errors.Is(err, os.ErrNotExist) {
		return Credentials{}, ErrNotFound
	}
	if err != nil {
		return Credentials{}, fmt.Errorf("credstore: read credentials: %w", err)
	}
	var creds Credentials
	if err := json.Unmarshal(data, &creds); err != nil {
		return Credentials{}, fmt.Errorf("credstore: unmarshal credentials: %w", err)
	}
	return creds, nil
}

func (p *plainFileSecrets) Delete() error {
	err := os.Remove(p.path)
	if err != nil && !errors.Is(err, os.ErrNotExist) {
		return fmt.Errorf("credstore: delete credentials: %w", err)
	}
	return nil
}
