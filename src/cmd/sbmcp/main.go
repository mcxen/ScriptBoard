// sbmcp is ScriptBoard's MCP server over stdio, for a client that starts a
// program. It works on the data folder directly, so it needs no running app:
// the app picks up the plugins and icons it writes within a second. The app
// itself offers the same thing, with a key, over HTTP (Settings › MCP).
//
//	go build -o sbmcp ./cmd/sbmcp
//	{ "mcpServers": { "scriptboard": { "command": "/path/to/sbmcp" } } }
package main

import (
	"log"
	"os"

	"scriptboard/src/internal/mcpsvc"
	"scriptboard/src/internal/plugins"
)

func main() {
	dir, err := plugins.DefaultDataDir()
	if err != nil {
		log.Fatal(err)
	}
	if err := (mcpsvc.Server{DataDir: dir}).Serve(os.Stdin, os.Stdout); err != nil {
		log.Fatal(err)
	}
}
