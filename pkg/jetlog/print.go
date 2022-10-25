package jetlog

import (
	"fmt"
	"io"
	"log"
	"strings"

	"github.com/briandowns/spinner"
	"github.com/fatih/color"
	"github.com/pkg/errors"
)

type logger struct {
	writer  io.Writer
	spinner *spinner.Spinner
}

func (l *logger) Write(p []byte) (n int, err error) {
	n, err = l.writer.Write(p)
	return n, errors.WithStack(err)
}

func (l *logger) HeaderPrintf(msg string, a ...any) {
	// TODO: This is a temporary fix, needs cleanup
	if l.spinner.Active() {
		l.spinner.Stop()
	}
	headerPrintfFunc := color.New(color.FgHiCyan, color.Bold).PrintfFunc()
	msg = "# " + msg + "\n"
	headerPrintfFunc(fmt.Sprintf(msg, a...))
}

func (l *logger) BoldPrintf(msg string, a ...any) {
	// TODO: This is a temporary fix, needs cleanup
	if l.spinner.Active() {
		l.spinner.Stop()
	}
	boldPrintfFunc := color.New(color.Bold).PrintfFunc()
	msg = "\n\t" + msg
	boldPrintfFunc(fmt.Sprintf(msg, a...))
}

func (l *logger) IndentedPrintf(msg string, a ...any) {
	msg = "\t" + msg
	printf(l.writer, fmt.Sprintf(msg, a...))
}

func (l *logger) IndentedPrintln(msg string, a ...any) {
	msg = "\t" + msg + "\n"
	printf(l.writer, fmt.Sprintf(msg, a...))
}

func (l *logger) WarningPrintf(msg string, a ...any) {
	printfFunc := color.New(color.FgHiYellow, color.Bold).PrintfFunc()
	msg = "WARNING: " + msg + "\n"
	printfFunc(fmt.Sprintf(msg, a...))
}

// WithSpinnerFuncPrint prints out a message and starts a spinner. closure() will then be
// executed, and the spinner stopped after it's done.
func (l *logger) WithSpinnerFuncPrint(closure func(), msg string) {
	if l.spinner.Active() { // from previous command.
		l.spinner.Stop()
	}
	l.spinner.Prefix = msg
	l.spinner.FinalMSG = "✔ " + msg + "\n"
	l.spinner.Start()
	defer l.spinner.Stop()

	closure()
}

// Print method specific for replacing logs with similar prefix with a spinner
func (l *logger) WithSpinnerPrintf(prefixes []string, spinnerMessage string, msg string, a ...any) {
	if hasPrefixes(msg, prefixes) {
		if !l.spinner.Active() {
			l.spinner.Prefix = spinnerMessage
			l.spinner.FinalMSG = spinnerMessage + "[DONE]\n"
			l.spinner.Start()
		}
	} else if l.spinner.Active() {
		l.spinner.Stop()
		l.IndentedPrintf(msg, a...)
	} else {
		l.IndentedPrintf(msg, a...)
	}
}

func (l *logger) Println(a ...any) {
	println(l.writer, a...)
}

func (l *logger) Print(msg string) {
	print(l.writer, msg)
}

func (l *logger) Printf(msg string, a ...any) {
	printf(l.writer, msg, a...)
}

func println(w io.Writer, lines ...any) {
	// mimic the functionality of fmt.Println()
	for i, line := range lines {
		if i > 0 {
			print(w, " ")
		}
		print(w, fmt.Sprintf("%v", line))
	}
	print(w, "\n")
}

func print(w io.Writer, msg string) {
	// TODO: Push cli output to a log file show point to it when an error happens
	_, err := w.Write([]byte(msg))
	if err != nil {
		log.Println(err)
	}
}

func printf(w io.Writer, msg string, a ...any) {
	print(w, fmt.Sprintf(msg, a...))
}

func hasPrefixes(msg string, prefixes []string) bool {
	for _, prefix := range prefixes {
		if strings.HasPrefix(msg, prefix) {
			return true
		}
	}
	return false
}
