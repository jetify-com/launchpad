package provider

type ErrorLogger interface {
	CaptureException(exception error)

	// DisplayException displays an error to the user. This is useful for custom error that padcli
	// would otherwise not know how to display in a user-friendly way. Returns true if the error
	// is displayed. If true, the caller can continue without doing further error handling.
	DisplayException(err error) bool
}

type NoOpLogger struct{}

var _ ErrorLogger = (*NoOpLogger)(nil)

func (l *NoOpLogger) CaptureException(err error) {}
func (l *NoOpLogger) DisplayException(err error) bool {
	return false
}
