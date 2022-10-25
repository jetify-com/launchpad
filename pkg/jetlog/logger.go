package jetlog

import (
	"context"
	"os"
	"time"

	"github.com/briandowns/spinner"
)

var outLogger *logger

// Logger wraps a writer and provides some helper functions for printing
// I'm not sure this is better than just using standard log functions directly
// and combining with a print helper but this helps transition away from
// jetlog.Logger(vc)
func Logger(ctx context.Context) *logger {
	if outLogger == nil {
		s := spinner.New(spinner.CharSets[26], 250*time.Millisecond)
		outLogger = &logger{os.Stdout, s}
	}
	return outLogger
}
