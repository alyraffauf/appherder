package appherder

import (
	"strings"

	"github.com/alyraffauf/goxdgdesktop/desktopexec"
)

func patchExecCommand(execCmd string, appimage string) string {
	tokens, err := desktopexec.Split(execCmd)
	if err != nil {
		return execCmd
	}
	if len(tokens) == 0 {
		return execCmd
	}

	envVars := []string{}
	executableIndex := 0
	if tokens[0] == "env" {
		executableIndex = 1
	}

	for executableIndex < len(tokens) && desktopexec.IsEnvAssignment(tokens[executableIndex]) {
		envVars = append(envVars, tokens[executableIndex])
		executableIndex++
	}

	args := []string{}
	for _, token := range tokens[executableIndex+1:] {
		if desktopexec.IsMetadataFieldCode(token) {
			continue
		}
		args = append(args, token)
	}

	hasDesktopIntegration := false
	for _, envVar := range envVars {
		if strings.HasPrefix(envVar, "DESKTOPINTEGRATION=") {
			hasDesktopIntegration = true
			break
		}
	}
	if !hasDesktopIntegration {
		envVars = append(envVars, "DESKTOPINTEGRATION=1")
	}

	cmd := append([]string{appimage}, args...)
	if len(envVars) > 0 {
		cmd = append([]string{"env"}, append(envVars, cmd...)...)
	}

	return desktopexec.Join(cmd...)
}

// execPath returns the executable from a desktop Exec/TryExec line.
func execPath(cmd string) string {
	return desktopexec.ExecutablePath(cmd)
}
