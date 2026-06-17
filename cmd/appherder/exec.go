package main

import (
	"strings"

	"github.com/kballard/go-shellquote"
)

func patchExecCommand(execCmd string, appimage string) string {
	tokens, err := shellquote.Split(execCmd)
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

	for executableIndex < len(tokens) && isEnvVar(tokens[executableIndex]) {
		envVars = append(envVars, tokens[executableIndex])
		executableIndex++
	}

	args := []string{}
	for _, token := range tokens[executableIndex+1:] {
		if isStrippedDesktopExecCode(token) {
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

	return shellquote.Join(cmd...)
}

func isEnvVar(token string) bool {
	return strings.Contains(token, "=") && !strings.HasPrefix(token, "/") && !strings.HasPrefix(token, "-")
}

// execPath returns the executable path from a desktop Exec/TryExec command,
// skipping a leading env and KEY=VALUE assignments.
func execPath(cmd string) string {
	tokens, err := shellquote.Split(cmd)
	if err != nil {
		return ""
	}
	i := 0
	if i < len(tokens) && tokens[i] == "env" {
		i++
	}
	for i < len(tokens) && isEnvVar(tokens[i]) {
		i++
	}
	if i < len(tokens) {
		return tokens[i]
	}
	return ""
}

func isStrippedDesktopExecCode(token string) bool {
	return token == "%i" || token == "%c" || token == "%k"
}
