//go:build js && wasm

// SPDX-License-Identifier: GPL-2.0-only

package main

import (
	"sync"

	"oblikovati.org/api/client"
	"oblikovati.org/api/wasmaddin"
	"oblikovati.org/spacemouse/bridge"
	"oblikovati.org/spacemouse/device"
)

const addInID = "com.oblikovati.3d-connexion"

type wasmSpaceMouse struct {
	mu     sync.Mutex
	engine *bridge.Engine
}

func (a *wasmSpaceMouse) ID() string       { return addInID }
func (a *wasmSpaceMouse) Manifest() string { return manifestJSON }

func (a *wasmSpaceMouse) Activate(caller client.Caller) error {
	a.mu.Lock()
	defer a.mu.Unlock()
	eng := bridge.NewEngine(caller, device.Open)
	a.engine = eng
	go func() { _ = eng.Setup() }()
	return nil
}

func (a *wasmSpaceMouse) Deactivate() error {
	a.mu.Lock()
	eng := a.engine
	a.engine = nil
	a.mu.Unlock()
	if eng != nil {
		eng.Stop()
	}
	return nil
}

func (a *wasmSpaceMouse) Notify(event []byte) {
	a.mu.Lock()
	eng := a.engine
	a.mu.Unlock()
	if eng != nil {
		eng.Notify(event)
	}
}

func main() {
	wasmaddin.Serve(&wasmSpaceMouse{})
}
