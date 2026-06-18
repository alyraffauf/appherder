package main

var upgradeUnits = []string{
	"appherder-upgrade.timer",
	"appherder-upgrade.service",
}

func enableAutoupgrade() error {
	if err := writeUnitFiles(upgradeUnits); err != nil {
		return err
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	return runSystemctl("enable", "--now", "appherder-upgrade.timer")
}

func disableAutoupgrade() error {
	if err := runSystemctl("disable", "--now", "appherder-upgrade.timer"); err != nil {
		return err
	}
	if err := removeUnitFiles(upgradeUnits); err != nil {
		return err
	}
	return runSystemctl("daemon-reload")
}
