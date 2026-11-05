package cmdutil

// UsageError is a command-line usage mistake: an unknown flag, a bad flag
// value or an unknown subcommand. main maps it to exit code 2.
type UsageError struct{ Err error }

func (e *UsageError) Error() string { return e.Err.Error() }

func (e *UsageError) Unwrap() error { return e.Err }
