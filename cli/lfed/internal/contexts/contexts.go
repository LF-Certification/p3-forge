// Package contexts is the compiled-in table of deployment contexts.
//
// A context names one Auth0 tenant and one education API gateway. The lfed CLI
// knows three: dev, staging, prod. Backend deployment suffixes MUST NOT
// appear here; a plugin that needs them owns that mapping.
package contexts

import (
	"errors"
	"fmt"
	"strings"
)

// Name is a context identifier accepted by -c and LFED_CONTEXT.
type Name string

// The three contexts the lfed CLI knows.
const (
	Dev     Name = "dev"
	Staging Name = "staging"
	Prod    Name = "prod"
)

// Context is one row of the table.
type Context struct {
	Name Name
	// Auth0Domain is the host for /oauth/device/code and /oauth/token.
	Auth0Domain string
	// Issuer is the expected iss claim of access tokens.
	Issuer string
	// Audience is the API identifier requested at login. It ends with "/".
	Audience string
	// ClientID is the Auth0 native client for the device code grant.
	ClientID string
}

// APIBaseURL returns the gateway base with no trailing slash.
func (c Context) APIBaseURL() string {
	return strings.TrimSuffix(c.Audience, "/")
}

// table is ordered dev, staging, prod for listings.
var table = []Context{
	{
		Name:        Dev,
		Auth0Domain: "linuxfoundation-dev.auth0.com",
		Issuer:      "https://linuxfoundation-dev.auth0.com/",
		Audience:    "https://api.dev.ed.linuxfoundation.org/",
		ClientID:    "0TN1OElqQY146vLEPdV5qfejRKpc9IAZ",
	},
	{
		Name:        Staging,
		Auth0Domain: "linuxfoundation-staging.auth0.com",
		Issuer:      "https://linuxfoundation-staging.auth0.com/",
		Audience:    "https://api.staging.ed.linuxfoundation.org/",
		ClientID:    "9XzXgDfAB9O7IoHqhBj5mg4VLvdBM8ci",
	},
	{
		Name:        Prod,
		Auth0Domain: "sso.linuxfoundation.org",
		Issuer:      "https://sso.linuxfoundation.org/",
		Audience:    "https://api.ed.linuxfoundation.org/",
		ClientID:    "kkCpM0c9zJ0vNZZDDOGqcyzocOBircOn",
	},
}

// ErrNoContext is returned by Resolve when no source names a context.
var ErrNoContext = errors.New("no context set; run `lfed context use <dev|staging|prod>`")

// All returns every context in listing order.
func All() []Context {
	out := make([]Context, len(table))
	copy(out, table)
	return out
}

// Names returns the accepted context names in listing order.
func Names() []string {
	out := make([]string, len(table))
	for i, c := range table {
		out[i] = string(c.Name)
	}
	return out
}

// Lookup returns the context called name.
func Lookup(name string) (Context, error) {
	for _, c := range table {
		if string(c.Name) == name {
			return c, nil
		}
	}
	return Context{}, fmt.Errorf("unknown context %q; expected one of %s", name, strings.Join(Names(), ", "))
}

// Resolve picks the context from the first non-empty source: the -c flag,
// LFED_CONTEXT, then the config file. It returns ErrNoContext when all are
// empty.
func Resolve(flag, env, stored string) (Context, error) {
	for _, candidate := range []string{flag, env, stored} {
		if candidate != "" {
			return Lookup(candidate)
		}
	}
	return Context{}, ErrNoContext
}
