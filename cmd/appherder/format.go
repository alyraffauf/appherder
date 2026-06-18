package main

import (
	"fmt"
	"io"
	"path/filepath"
	"text/tabwriter"

	"github.com/alyraffauf/appherder/internal/appherder"
)

func printAppList(out io.Writer, infos []appherder.AppInfo) {
	tw := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	fmt.Fprintln(tw, "NAME\tFILENAME\tVERSION\tSIZE\tSOURCE\tSIGNATURE")
	for _, info := range infos {
		size := "-"
		if info.Size > 0 {
			size = humanSize(info.Size)
		}
		fmt.Fprintf(tw, "%s\t%s\t%s\t%s\t%s\t%s\n",
			info.Name, orDash(info.Filename), orDash(info.Version), size, orDash(info.Source), info.Signature)
	}
	tw.Flush()
}

func printSyncResult(out io.Writer, result appherder.SyncResult) {
	for _, inst := range result.Installs {
		if inst.Err != nil {
			fmt.Fprintf(out, "skipped %s: %v\n", filepath.Base(inst.File), inst.Err)
			continue
		}
		if inst.New {
			fmt.Fprintf(out, "installed %s\n", inst.AppName)
		}
	}
	for _, rem := range result.Removals {
		if rem.Err != nil {
			fmt.Fprintf(out, "skipped removing %s: %v\n", rem.AppName, rem.Err)
			continue
		}
		fmt.Fprintf(out, "removed %s\n", rem.AppName)
	}
}

func printUpgradeChecks(out io.Writer, checks []appherder.UpgradeCheck) {
	available := 0
	for _, check := range checks {
		if check.Err != nil {
			fmt.Fprintf(out, "skipped %s: %v\n", check.Name, check.Err)
			continue
		}
		if check.NoSource || !check.Available {
			continue
		}
		available++
		fmt.Fprintf(out, "%s: update available (%s)\n", check.Name, check.Release.Version)
	}
	if available == 0 {
		fmt.Fprintln(out, "everything is up to date")
	}
}

func printUpgradeApplied(out io.Writer, checks []appherder.UpgradeCheck, applied []appherder.UpgradeApplied) {
	for _, app := range applied {
		if app.Err != nil {
			fmt.Fprintf(out, "skipped %s: %v\n", app.Name, app.Err)
			continue
		}
		fmt.Fprintf(out, "upgraded %s to %s\n", app.Name, app.Version)
	}
	if len(applied) == 0 {
		fmt.Fprintln(out, "everything is up to date")
	}
}

func orDash(s string) string {
	if s == "" {
		return "-"
	}
	return s
}

func humanSize(bytes int64) string {
	const unit = 1024
	if bytes < unit {
		return fmt.Sprintf("%d B", bytes)
	}
	div, exp := int64(unit), 0
	for scaled := bytes / unit; scaled >= unit; scaled /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %cB", float64(bytes)/float64(div), "KMGTPE"[exp])
}
