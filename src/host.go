package main

import (
	"sync"

	"github.com/branchkit/plugin-sdk-go"
)

// Host is what the handlers need: the platform handle and the one piece of
// tab state the Import tab shows back (the last import's outcome). Handlers
// are methods on it, so their dependencies are visible in the signature.
type Host struct {
	plugin *branchkit.Plugin

	importMu         sync.Mutex
	lastImportResult string
}

func newHost(p *branchkit.Plugin) *Host { return &Host{plugin: p} }
