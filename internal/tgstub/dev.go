package tgstub

import (
	"fmt"
	"net"
	"strconv"
	"strings"
)

const DevToken = "123456789:AAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAAA"

func DevEnvironment(baseURL string) map[string]string {
	return map[string]string{
		"MGKE_TELEGRAM_API_BASE_URL":    baseURL,
		"MGKE_TELEGRAM_TOKEN":           DevToken,
		"MGKE_TELEGRAM_WEBHOOK_ENABLED": "false",
	}
}

func LoopbackBaseURL(listen string) (string, error) {
	address := strings.TrimSpace(listen)
	if address == "" {
		return "", fmt.Errorf("stub listen address is empty, want host:port")
	}

	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return "", fmt.Errorf("stub listen address %q must be host:port: %v", address, err)
	}
	if host == "" {
		host = "127.0.0.1"
	}
	if !loopbackHost(host) {
		return "", fmt.Errorf("stub listen address %q is not loopback: the stub must never be reachable from another machine", address)
	}

	number, err := strconv.Atoi(port)
	if err != nil || number < 1 || number > 65535 {
		return "", fmt.Errorf("stub listen address %q needs a port between 1 and 65535", address)
	}

	return "http://" + net.JoinHostPort(strings.Trim(host, "[]"), port), nil
}

func loopbackHost(host string) bool {
	if strings.EqualFold(host, "localhost") {
		return true
	}

	ip := net.ParseIP(strings.Trim(host, "[]"))
	return ip != nil && ip.IsLoopback()
}
