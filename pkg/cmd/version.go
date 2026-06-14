package cmd

import (
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/alexremn/kubectl-fieldlord/internal/buildinfo"
)

func newVersionCmd() *cobra.Command {
	return &cobra.Command{
		Use:           "version",
		Short:         "Print version information",
		Args:          cobra.NoArgs,
		SilenceUsage:  true,
		SilenceErrors: true,
		RunE: func(cmd *cobra.Command, _ []string) error {
			_, err := fmt.Fprintf(cmd.OutOrStdout(),
				"kubectl-fieldlord %s\n  commit: %s\n  built:  %s\n  go:     %s %s/%s\n",
				buildinfo.Version, buildinfo.Commit, buildinfo.Date,
				runtime.Version(), runtime.GOOS, runtime.GOARCH,
			)
			return err
		},
	}
}
