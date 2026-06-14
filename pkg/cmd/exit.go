package cmd

// ExitError carries a process exit code out of a command's RunE.
// Code 2 with a nil Err means "findings present" (CI gate) and is printed quietly.
type ExitError struct {
	Code int
	Err  error
}

func (e *ExitError) Error() string {
	if e.Err == nil {
		return ""
	}
	return e.Err.Error()
}

func (e *ExitError) Unwrap() error { return e.Err }
