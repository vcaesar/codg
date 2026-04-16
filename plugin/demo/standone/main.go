// Package main builds the demo plugin as a Go shared object.
//
// Build with:
//
//	go build -buildmode=plugin -o demo.so .
//
// Install with:
//
//	codg install ./pkg/plugin/demo/standalone
//
// The resulting .so file exports a CodgPlugin symbol that the Codg
// plugin system loads at runtime. This allows the demo plugin to run
// independently without being compiled into the Codg binary.
package main

import (
	"github.com/vcaesar/codg/plugin/demo"

	"github.com/vcaesar/codg/plugin"
)

// CodgPlugin is the exported symbol that the Codg plugin loader looks
// up. It must implement plugin.Plugin.
var CodgPlugin plugin.Plugin = &demo.Plugin{} //nolint:gochecknoglobals
