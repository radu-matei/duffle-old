package main

import (
	"errors"
	"io"

	"github.com/deis/duffle/pkg/repo"

	"github.com/spf13/cobra"
)

func newRepoGenerateCmd(w io.Writer) *cobra.Command {
	var url string

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "generate repositories",
		RunE: func(cmd *cobra.Command, args []string) error {
			if len(args) != 1 {
				return errors.New("This command requires at least one argument: DIRNAME (name of the bundle directory to generate a repository for).\nValid inputs:\n\t$ duffle repo generate DIRNAME")
			}

			return generateRepo(args[0], url)
		},
	}

	f := cmd.Flags()
	f.StringVar(&url, "url", "", "url of chart repository")

	return cmd
}

func generateRepo(dir, url string) error {
	return repo.GenerateFromDirectory(dir, url)
}
