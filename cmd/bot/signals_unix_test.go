//go:build !windows

package main

import (
	"syscall"
	"testing"
)

func TestStopSignalsCoverUnixTerminationSignals(t *testing.T) {
	for _, want := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM, syscall.SIGHUP, syscall.SIGQUIT} {
		if !containsSignal(stopSignals(), want) {
			t.Errorf("stopSignals() = %v, want it to include %v", signalNames(stopSignals()), want)
		}
	}
}
