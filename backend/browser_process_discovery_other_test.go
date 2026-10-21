//go:build !windows
// +build !windows

package backend

import "testing"

func TestParseBrowserProcessListOutputMatchesUserDataDir(t *testing.T) {
	target := "/tmp/ant-browser/profile one"
	output := `
  101 /usr/bin/chrome --user-data-dir="/tmp/ant-browser/profile one" --remote-debugging-port=9222
  102 /usr/bin/chrome --user-data-dir=/tmp/ant-browser/other --remote-debugging-port=9223
  103 /usr/bin/chrome --user-data-dir "/tmp/ant-browser/profile one" --type=renderer
`

	processes := parseBrowserProcessListOutput(output, target)
	if len(processes) != 2 {
		t.Fatalf("matched processes = %#v, want two entries", processes)
	}
	if processes[0].PID != 101 || processes[0].DebugPort != 9222 {
		t.Fatalf("first process = %#v, want pid 101 and port 9222", processes[0])
	}
	if processes[1].PID != 103 {
		t.Fatalf("second process = %#v, want pid 103", processes[1])
	}
}

func TestParseProcessListLineRejectsInvalidRows(t *testing.T) {
	for _, input := range []string{"", "PID COMMAND", "abc /usr/bin/chrome", "101"} {
		if pid, commandLine, ok := parseProcessListLine(input); ok || pid != 0 || commandLine != "" {
			t.Fatalf("parseProcessListLine(%q) = (%d, %q, %t), want invalid", input, pid, commandLine, ok)
		}
	}
}
