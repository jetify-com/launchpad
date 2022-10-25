package errorutil

import "github.com/pkg/errors"

func EarliestStackTrace(err error) errors.StackTrace {
	type stackTracer interface {
		StackTrace() errors.StackTrace
	}

	type causer interface {
		Cause() error
	}

	var st stackTracer
	var c causer
	var earliestStackTrace errors.StackTrace

	for err != nil {
		if errors.As(err, &st) {
			earliestStackTrace = st.StackTrace()
		}

		if !errors.As(err, &c) {
			break
		}
		err = c.Cause()
	}

	return earliestStackTrace
}
