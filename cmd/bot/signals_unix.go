//go:build !windows

package main

import (
	"os"
	"syscall"
)

func stopSignals() []os.Signal {
	return []os.Signal{
		syscall.SIGINT,
		syscall.SIGTERM,
		syscall.SIGHUP,
		syscall.SIGQUIT,
	}
}
