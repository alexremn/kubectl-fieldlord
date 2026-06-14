package main

import (
	"errors"
	"fmt"
	"os"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/alexremn/kubectl-fieldlord/pkg/cmd"
)

func main() {
	streams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	os.Exit(run(os.Args[1:], streams))
}

// run is separate (and parameterized) so os.Exit does not skip command defers
// and so the exit-code mapping can be unit-tested with injected args/streams.
func run(args []string, streams genericiooptions.IOStreams) int {
	root := cmd.NewRootCmd(streams)
	root.SetArgs(args)

	err := root.Execute()
	if err == nil {
		return 0
	}
	var ee *cmd.ExitError
	if errors.As(err, &ee) {
		if ee.Err != nil {
			fmt.Fprintln(streams.ErrOut, "Error:", ee.Err)
		}
		return ee.Code
	}
	fmt.Fprintln(streams.ErrOut, "Error:", err)
	return 1
}
