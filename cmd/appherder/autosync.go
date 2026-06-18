package main

var syncUnits = []string{
	"appherder-sync.path",
	"appherder-sync.service",
}

func enableAutosync() error {
	if err := writeUnitFiles(syncUnits); err != nil {
		return err
	}
	if err := runSystemctl("daemon-reload"); err != nil {
		return err
	}
	return runSystemctl("enable", "--now", "appherder-sync.path")
}

func disableAutosync() error {
	if err := runSystemctl("disable", "--now", "appherder-sync.path"); err != nil {
		return err
	}
	if err := removeUnitFiles(syncUnits); err != nil {
		return err
	}
	return runSystemctl("daemon-reload")
}
