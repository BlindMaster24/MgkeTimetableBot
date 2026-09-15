package main

import (
	"syscall"
	"testing"
)

func TestStopSignalsUseSignalsWindowsCanDeliver(t *testing.T) {
	for _, want := range []syscall.Signal{syscall.SIGINT, syscall.SIGTERM} {
		if !containsSignal(stopSignals(), want) {
			t.Errorf("stopSignals() = %v, want it to include %v", signalNames(stopSignals()), want)
		}
	}
}
