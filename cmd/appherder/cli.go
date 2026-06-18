package main

import (
	"fmt"
	"io"

	"github.com/spf13/cobra"
)

func newRootCommand(a app, stdout io.Writer, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "appherder",
		Short: "A herder for your AppImages",
		Long: "appherder installs AppImages so they show up in your application menu with their\n" +
			"real name and icon, instead of sitting in a folder doing nothing. Drop AppImages in\n" +
			"~/AppImages and appherder keeps everything in sync.",
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(newInstallCommand(a), newUninstallCommand(a), newListCommand(a), newSyncCommand(a), newMigrateCommand(a), newUpgradeCommand(a))
	return cmd
}

func newInstallCommand(a app) *cobra.Command {
	return &cobra.Command{
		Use:   "install APPIMAGE",
		Short: "Install an AppImage",
		Long:  "Copies an AppImage into ~/AppImages and creates a launcher for it with the app's\nreal name and icon. You can delete the original download afterward.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			name, err := a.install(args[0])
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", name)
			return nil
		},
	}
}

func newUninstallCommand(a app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "uninstall APP|APPIMAGE",
		Short: "Uninstall an AppImage",
		Long:  "Removes an AppImage and its launcher and icon. Give the name as it appears in\n~/AppImages (without .appimage), or the full path.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.uninstall(args[0], force); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", normalizeAppName(args[0]))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Remove the app even if appherder didn't install it")
	return cmd
}

func newListCommand(a app) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show managed AppImages",
		Long:  "Shows every app appherder is managing: its display name, version, file size,\nand where it checks for updates.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.list(cmd.OutOrStdout())
		},
	}
}

func newSyncCommand(a app) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Match launchers to what's in ~/AppImages",
		Long: "Installs any AppImages in ~/AppImages that don't have a launcher yet, and removes\n" +
			"launchers whose AppImage is gone. Run this after adding or removing files from\n" +
			"~/AppImages.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.sync(cmd.Context(), cmd.OutOrStdout(), force)
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Also remove launchers appherder didn't install, when their AppImage is gone")
	return cmd
}

func newMigrateCommand(a app) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Adopt apps from another tool",
		Long: "Like sync --force: adopts launchers another tool created and removes the ones\n" +
			"whose AppImage is missing. Safe to run anytime.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.sync(cmd.Context(), cmd.OutOrStdout(), true)
		},
	}
}

func newUpgradeCommand(a app) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Download and install updates",
		Long: "Checks for and installs updates. appherder reads the update info baked into each\n" +
			"AppImage and fetches the latest from GitHub, GitLab, zsync, or a static URL.\n" +
			"Apps with no update info or already current are skipped silently.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			return a.upgrade(cmd.Context(), cmd.OutOrStdout(), check)
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Report available updates without installing them")
	return cmd
}
