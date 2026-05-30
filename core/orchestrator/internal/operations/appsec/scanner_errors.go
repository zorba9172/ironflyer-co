package appsec

import (
	"fmt"
	"strings"

	"ironflyer/core/orchestrator/internal/operations/runtime"
)

func scannerTransportError(tool string, res runtime.ExecResult, err error) error {
	if err != nil {
		return fmt.Errorf("%s scanner execution failed: %w", tool, err)
	}
	if res.TimedOut {
		return fmt.Errorf("%s scanner timed out%s", tool, scannerStderrSuffix(res))
	}
	return nil
}

func scannerExitError(tool string, res runtime.ExecResult) error {
	if res.ExitCode == 0 {
		return nil
	}
	return fmt.Errorf("%s scanner exited with code %d%s", tool, res.ExitCode, scannerStderrSuffix(res))
}

func scannerParseError(tool string, err error) error {
	if err == nil {
		return nil
	}
	return fmt.Errorf("%s scanner output parse failed: %w", tool, err)
}

func scannerStderrSuffix(res runtime.ExecResult) string {
	msg := strings.TrimSpace(res.Stderr)
	if msg == "" {
		return ""
	}
	msg = strings.ReplaceAll(msg, "\n", " ")
	if len(msg) > 240 {
		msg = msg[:240]
	}
	return ": " + msg
}
