package main

import (
	"log"
	"os"
	"strings"

	"github.com/nikochiko/tcpchat/client"
	"github.com/nikochiko/tcpchat/server"
)

func main() {
	if len(os.Args) < 3 {
		log.Fatalf("Usage: %s [client|server] <host>:<port>\n", os.Args[0])
	}

	service := os.Args[2]

	switch component := os.Args[1]; strings.ToLower(component) {
	case "client":
		client.Connect(service)
	case "server":
		server.Listen(service)
	default:
		log.Fatalf("Unrecognised component %s\n", component)
	}
}
