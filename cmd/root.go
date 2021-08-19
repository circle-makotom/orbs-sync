package cmd

import (
	"fmt"

	"github.com/spf13/cobra"
)

var (
	BuildName       = "\b"
	BuildAnnotation = "git"

	debug = false
)

func Execute() error {
	cmd := &cobra.Command{
		Use:          "orbs-sync",
		Version:      fmt.Sprintf("%s (%s)", BuildName, BuildAnnotation),
		SilenceUsage: true,
	}

	cmd.PersistentFlags().BoolVar(&debug, "debug", false, "Show debugging information")

	cmd.AddCommand(cmdCollect())
	cmd.AddCommand(cmdResolveDependencies())
	cmd.AddCommand(cmdBulkImport())
	cmd.AddCommand(cmdSync())

	return cmd.Execute()
}
