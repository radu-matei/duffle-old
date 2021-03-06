package main

import (
	"io"

	"github.com/spf13/cobra"
)

// TODO
func newRootCmd(w io.Writer) *cobra.Command {
	const usage = `The CNAB installer`

	cmd := &cobra.Command{
		Use:   "duffle",
		Short: usage,
		Long:  usage,
		Run: func(cmd *cobra.Command, args []string) {
			unimplemented("duffle")
		},
	}

	cmd.AddCommand(newBuildCmd(w))
	cmd.AddCommand(newInitCmd(w))
	cmd.AddCommand(newPullCmd(w))
	cmd.AddCommand(newPushCmd(w))
	cmd.AddCommand(newRunCmd(w))

	return cmd
}
