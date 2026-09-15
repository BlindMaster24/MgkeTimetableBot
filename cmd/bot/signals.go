package main

import "os"

func signalNames(signals []os.Signal) []string {
	names := make([]string, 0, len(signals))
	for _, sig := range signals {
		names = append(names, sig.String())
	}
	return names
}
