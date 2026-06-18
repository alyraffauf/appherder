package main

import (
	"fmt"
	"io"
	"net/url"

	"github.com/alyraffauf/appherder/internal/appherder"
	"github.com/spf13/cobra"
)

func newRootCommand(a appherder.App, stdout io.Writer, stderr io.Writer) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "appherder",
		Short: "A shepherd for your AppImages",
		Long: "appherder manages your AppImages for you. Drop them in ~/AppImages and they show\n" +
			"up in your menu like any other app, with their real name and icon. New versions\n" +
			"replace the old ones. Add or remove AppImages and appherder sorts it out,\n" +
			"optionally on its own.",
		Version:       version,
		SilenceUsage:  true,
		SilenceErrors: true,
	}
	cmd.SetOut(stdout)
	cmd.SetErr(stderr)
	cmd.AddCommand(
		newInstallCommand(a),
		newUninstallCommand(a),
		newListCommand(a),
		newSyncCommand(a),
		newMigrateCommand(a),
		newUpgradeCommand(a),
		newRollbackCommand(a),
		newAutosyncCommand(),
		newAutoupgradeCommand(),
	)
	return cmd
}

func newInstallCommand(a appherder.App) *cobra.Command {
	return &cobra.Command{
		Use:   "install APPIMAGE|URL",
		Short: "Install an AppImage from a file or URL",
		Long: "Copies an AppImage into ~/AppImages and creates a launcher for it with the app's\n" +
			"real name and icon. You can delete the original download afterward.\n\n" +
			"Accepts a local file path or an HTTP/HTTPS URL to download directly.",
		Args: cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			arg := args[0]
			var name string
			var err error
			if isURL(arg) {
				fmt.Fprintf(cmd.OutOrStdout(), "downloading %s...\n", arg)
				name, err = a.InstallFromURL(cmd.Context(), arg)
			} else {
				name, err = a.Install(arg)
			}
			if err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "installed %s\n", name)
			return nil
		},
	}
}

func isURL(s string) bool {
	parsed, err := url.Parse(s)
	return err == nil && (parsed.Scheme == "http" || parsed.Scheme == "https") && parsed.Host != ""
}

func newUninstallCommand(a appherder.App) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "uninstall APP|APPIMAGE",
		Short: "Uninstall an AppImage",
		Long:  "Removes an AppImage and its launcher and icon. Give the name as it appears in\n~/AppImages (without .appimage), or the full path.",
		Args:  cobra.ExactArgs(1),
		RunE: func(cmd *cobra.Command, args []string) error {
			if err := a.Uninstall(args[0], force); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "removed %s\n", appherder.NormalizeAppName(args[0]))
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Remove the app even if appherder didn't install it")
	return cmd
}

func newListCommand(a appherder.App) *cobra.Command {
	return &cobra.Command{
		Use:   "list",
		Short: "Show managed AppImages",
		Long:  "Shows every app appherder is managing: its display name, version, file size,\nand where it checks for updates.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			infos, err := a.List()
			if err != nil {
				return err
			}
			printAppList(cmd.OutOrStdout(), infos)
			return nil
		},
	}
}

func newSyncCommand(a appherder.App) *cobra.Command {
	var force bool
	cmd := &cobra.Command{
		Use:   "sync",
		Short: "Match launchers to what's in ~/AppImages",
		Long: "Installs any AppImages in ~/AppImages that don't have a launcher yet, and removes\n" +
			"launchers whose AppImage is gone. Run this after adding or removing files from\n" +
			"~/AppImages.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.Sync(cmd.Context(), force)
			if err != nil {
				return err
			}
			printSyncResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
	cmd.Flags().BoolVarP(&force, "force", "f", false,
		"Also remove launchers appherder didn't install, when their AppImage is gone")
	return cmd
}

func newMigrateCommand(a appherder.App) *cobra.Command {
	return &cobra.Command{
		Use:   "migrate",
		Short: "Adopt apps from another tool",
		Long: "Like sync --force: adopts launchers another tool created and removes the ones\n" +
			"whose AppImage is missing. Safe to run anytime.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			result, err := a.Sync(cmd.Context(), true)
			if err != nil {
				return err
			}
			printSyncResult(cmd.OutOrStdout(), result)
			return nil
		},
	}
}

func newUpgradeCommand(a appherder.App) *cobra.Command {
	var check bool
	cmd := &cobra.Command{
		Use:   "upgrade",
		Short: "Download and install updates",
		Long: "Checks for and installs updates. appherder reads the update info baked into each\n" +
			"AppImage and fetches the latest from GitHub, GitLab, zsync, or a static URL.\n" +
			"Apps with no update info or already current are skipped silently.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			out := cmd.OutOrStdout()
			checks, err := a.CheckUpgrades(cmd.Context())
			if err != nil {
				return err
			}
			if check {
				printUpgradeChecks(out, checks)
				return nil
			}
			applied := a.ApplyUpgrades(cmd.Context(), checks)
			printUpgradeApplied(out, checks, applied)
			return nil
		},
	}
	cmd.Flags().BoolVar(&check, "check", false, "Report available updates without installing them")
	return cmd
}

func newRollbackCommand(a appherder.App) *cobra.Command {
	return &cobra.Command{
		Use:   "rollback APP [VERSION]",
		Short: "Restore a previously saved version",
		Long: "Rolls the app back to a saved version. Without a version, the most recently\n" +
			"saved one is restored. appherder saves the current version whenever it is\n" +
			"replaced by an install or upgrade.",
		Args: cobra.RangeArgs(1, 2),
		RunE: func(cmd *cobra.Command, args []string) error {
			version := ""
			if len(args) > 1 {
				version = args[1]
			}
			if err := a.Rollback(args[0], version); err != nil {
				return err
			}
			fmt.Fprintf(cmd.OutOrStdout(), "rolled back %s\n", appherder.NormalizeAppName(args[0]))
			return nil
		},
	}
}

func newAutoCommand(use, short, long, offHelp string, enable, disable func() error) *cobra.Command {
	var off bool
	cmd := &cobra.Command{
		Use:   use,
		Short: short,
		Long:  long,
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if off {
				return disable()
			}
			return enable()
		},
	}
	cmd.Flags().BoolVar(&off, "off", false, offHelp)
	return cmd
}

func newAutosyncCommand() *cobra.Command {
	return newAutoCommand(
		"autosync",
		"Enable or disable automatic sync when AppImages change",
		"Installs a systemd user unit that watches ~/AppImages and runs sync whenever a\n"+
			"file is added or removed. No root required.",
		"Disable and remove the autosync watcher",
		func() error { return enableUnits(syncUnits) },
		func() error { return disableUnits(syncUnits) },
	)
}

func newAutoupgradeCommand() *cobra.Command {
	return newAutoCommand(
		"autoupgrade",
		"Enable or disable daily automatic upgrades",
		"Installs a systemd user timer that checks for and installs AppImage updates\n"+
			"once a day. No root required.",
		"Disable and remove the upgrade timer",
		func() error { return enableUnits(upgradeUnits) },
		func() error { return disableUnits(upgradeUnits) },
	)
}
