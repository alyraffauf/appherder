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
	cmd.AddCommand(newInstallCommand(a), newUninstallCommand(a), newSyncCommand(a), newMigrateCommand(a), newUpgradeCommand(a))
	return cmd
}

func newUpgradeCommand(a app) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Download and install updates for your AppImages",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.upgrade(cmd.Context(), cmd.OutOrStdout(), check)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Report available updates without installing them")
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

func newUninstallCommand(a app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "uninstall APP|APPIMAGE",
		Short: "Uninstall an AppImage",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.uninstall(args[0], force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Remove the app even if appherder didn't install it")
	return cmd
}

func newSyncCommand(a app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Reconcile ~/AppImages with installed applications",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.sync(cmd.OutOrStdout(), force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Also remove launchers appherder didn't install, when their AppImage is gone")
	return cmd
}

func newMigrateCommand(a app) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Adopt launchers from another tool and remove their broken orphans",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.sync(cmd.OutOrStdout(), true)
		},
	}
}
