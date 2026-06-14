package main

import (
	"errors"
	"fmt"
	"os"

	"k8s.io/cli-runtime/pkg/genericiooptions"

	"github.com/alexremn/kubectl-fieldlord/pkg/cmd"
)

func main() { os.Exit(run()) }

// run is separate so os.Exit does not skip command defers.
func run() int {
	streams := genericiooptions.IOStreams{In: os.Stdin, Out: os.Stdout, ErrOut: os.Stderr}
	root := cmd.NewRootCmd(streams)

	err := root.Execute()
	if err == nil {
		return 0
	}
	var ee *cmd.ExitError
	if errors.As(err, &ee) {
		if ee.Err != nil {
			fmt.Fprintln(os.Stderr, "Error:", ee.Err)
		}
		return ee.Code
	}
	fmt.Fprintln(os.Stderr, "Error:", err)
	return 1
}
