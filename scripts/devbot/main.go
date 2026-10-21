package main

import (
	"context"
	"flag"
	"fmt"
	"net"
	"net/http"
	"os"
	"strings"
	"time"

	"github.com/blindmaster24/MgkeTimetableBot/internal/app"
	"github.com/blindmaster24/MgkeTimetableBot/internal/build"
	"github.com/blindmaster24/MgkeTimetableBot/internal/tgstub"
)

var (
	version = "dev"
	commit  = "local"
	date    = "unknown"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18090", "loopback address of the stub Bot API")
	configPath := flag.String("config", "configs/config.dev.yaml", "config file the bot runs with")
	send := flag.String("send", "", "message text to queue for the bot before it starts polling")
	command := flag.String("command", "", "command name to queue as /name, e.g. start (no slash, so Git Bash keeps it intact)")
	flag.Parse()

	if err := run(*listen, *configPath, *send, *command); err != nil {
		fmt.Fprintln(os.Stderr, "devbot: "+err.Error())
		os.Exit(1)
	}
}

func run(listen, configPath, send, command string) error {
	base, err := tgstub.LoopbackBaseURL(listen)
	if err != nil {
		return err
	}
	if err := os.MkdirAll(".dev", 0o755); err != nil {
		return fmt.Errorf("create the dev state directory: %w", err)
	}

	stub := tgstub.New()
	listener, err := net.Listen("tcp", listen)
	if err != nil {
		return fmt.Errorf("listen on %s: %w", listen, err)
	}

	server := &http.Server{Handler: stub, ReadHeaderTimeout: 10 * time.Second}
	served := make(chan error, 1)
	go func() {
		if err := server.Serve(listener); err != nil && err != http.ErrServerClosed {
			served <- err
		}
	}()
	defer func() { _ = server.Close() }()

	for name, value := range tgstub.DevEnvironment(base) {
		if err := os.Setenv(name, value); err != nil {
			return fmt.Errorf("set %s: %w", name, err)
		}
	}

	fmt.Printf("stub Bot API:   %s\n", base)
	fmt.Printf("recorded calls: %s%s\n", base, "/_stub/calls")
	fmt.Printf("queue an update: curl -X POST %s%s -d '{\"text\":\"/start\"}'\n", base, "/_stub/updates")
	fmt.Println("telegram traffic stays on this machine: the bot uses a stub token and the local API base URL")

	if send != "" {
		fmt.Printf("queued update %d %q\n", stub.QueueUpdate(send, 0, 0), send)
	}
	if command != "" {
		text := "/" + strings.TrimPrefix(strings.TrimSpace(command), "/")
		fmt.Printf("queued update %d %q\n", stub.QueueUpdate(text, 0, 0), text)
	}

	select {
	case err := <-served:
		return fmt.Errorf("stub server: %w", err)
	default:
	}

	code := app.Run(context.Background(), app.ResolveConfigPath(configPath), build.New(version, commit, date))
	fmt.Printf("bot stopped, %d Bot API calls recorded in %s%s\n", len(stub.Calls()), base, "/_stub/calls")

	if code != 0 {
		return fmt.Errorf("bot exited with code %d", code)
	}
	return nil
}
