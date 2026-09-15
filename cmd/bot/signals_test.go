package main

import (
	"os"
	"testing"
)

func TestStopSignalsAlwaysIncludeInterrupt(t *testing.T) {
	if !containsSignal(stopSignals(), os.Interrupt) {
		t.Errorf("stopSignals() = %v, want it to include os.Interrupt", signalNames(stopSignals()))
	}
}

func TestSignalNamesRenderEverySignal(t *testing.T) {
	signals := stopSignals()
	names := signalNames(signals)

	if len(names) != len(signals) {
		t.Fatalf("signalNames() = %v, want one name per signal (%d)", names, len(signals))
	}
	for _, name := range names {
		if name == "" {
			t.Errorf("a stop signal has no name: %v", names)
		}
	}
}

func containsSignal(signals []os.Signal, want os.Signal) bool {
	for _, sig := range signals {
		if sig == want {
			return true
		}
	}
	return false
}
