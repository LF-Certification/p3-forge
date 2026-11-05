// Package build holds values set at link time.
package build

// Version is the lfed CLI semver. The Makefile sets it with -ldflags. Plugins
// compare their requires_lfed range against this value.
var Version = "0.1.0-dev"
