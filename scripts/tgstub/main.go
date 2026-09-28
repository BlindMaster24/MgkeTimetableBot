package main

import (
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"

	"github.com/blindmaster24/MgkeTimetableBot/internal/tgstub"
)

func main() {
	listen := flag.String("listen", "127.0.0.1:18090", "address to serve the stub Bot API on")
	flag.Parse()

	stub := tgstub.New()

	fmt.Printf("stub Bot API on http://%s (calls: http://%s%s)\n", *listen, *listen, "/_stub/calls")
	if err := http.ListenAndServe(*listen, stub); err != nil {
		log.New(os.Stderr, "tgstub: ", 0).Printf("serve: %v", err)
		os.Exit(1)
	}
}
