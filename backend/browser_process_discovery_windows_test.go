//go:build windows
// +build windows

package backend

import (
	"strings"
	"testing"
)

func TestRunPowerShellJSONBindsNamedArguments(t *testing.T) {
	output, err := runPowerShellJSON(`param([string]$Value)
[pscustomobject]@{ value = $Value } | ConvertTo-Json -Compress`, "-Value", "bound value")
	if err != nil {
		t.Fatalf("runPowerShellJSON returned error: %v", err)
	}
	if value := strings.TrimSpace(string(output)); value != `{"value":"bound value"}` {
		t.Fatalf("PowerShell parameter binding output = %q, want %q", value, `{"value":"bound value"}`)
	}
}
