package main

import (
	"io"

	"github.com/spf13/cobra"
)

func newRootCommand(a app, stdout io.Writer, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:           "appherder",
		Short:         "Manage AppImages with desktop integration",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newInstallCommand(a))
	return cmd
}

func newInstallCommand(a app) *cobra.Command {
	return &cobra.Command{
		Use:   "install APPIMAGE",
		Short: "Install an AppImage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.install(args[0])
		},
	}
}
