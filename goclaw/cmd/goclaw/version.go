package main

// Version is the build version of goclaw. It is set at build time via:
//
//	go build -ldflags "-X main.Version=v1.2.3" ./cmd/goclaw
//
// When built without -ldflags it defaults to "dev".
var Version = "dev"
