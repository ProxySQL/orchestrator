package config

import "testing"

func TestFailRecoveryOnFailedPostFailoverProcessesDefault(t *testing.T) {
	c := newConfiguration()
	if c.FailRecoveryOnFailedPostFailoverProcesses {
		t.Fatal("default must be false for backward compatibility")
	}
}
