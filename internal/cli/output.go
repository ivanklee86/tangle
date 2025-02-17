package cli

import (
	"fmt"
	"io"

	"github.com/jedib0t/go-pretty/v6/text"
)

const (
	headerPrefix = "tangle-cli"
)

// printToStream prints a generic message to a stream (stdout/stderror) in color.
func printToStream(stream io.Writer, msg interface{}) {
	_, err := fmt.Fprintf(stream, "%v\n", msg)
	if err != nil {
		panic(err)
	}
}

// printToStreamWithColor prints a message after wrapping it in ANSI color codes.
func printToStreamWithColor(stream io.Writer, color text.Color, msg interface{}) {
	_, err := fmt.Fprint(stream, color.Sprintf("%v\n", msg))
	if err != nil {
		panic(err)
	}
}

// OutputHeading prints a header to stdout.
func (t *TangleCLI) OutputHeading(msg interface{}) {
	printToStreamWithColor(t.Out, text.FgHiCyan, fmt.Sprintf("%v: %v", headerPrefix, msg))
}

// Output prints a normal message to stdout.
func (t *TangleCLI) Output(msg interface{}) {
	printToStream(t.Out, msg)
}

// Error pritns an error to stderr and exits with error code 1.
func (t *TangleCLI) Error(msg interface{}) {
	printToStreamWithColor(t.Err, text.FgHiRed, fmt.Sprintf("Error: %v\n", msg))
}
