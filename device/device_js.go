//go:build js && wasm && !nodevice

// SPDX-License-Identifier: GPL-2.0-only

package device

import (
	"sync"
	"syscall/js"
	"time"

	"oblikovati.org/api/wasmaddin"
)

// Open returns a browser SpaceMouse reader fed by WebHID input reports forwarded
// from the main thread over `globalThis.__obkHost.onChannel("webhid:report", ...)`.
func Open() (Device, error) {
	d := &webHIDDevice{
		ch: make(chan MotionSample, 32),
	}
	host := wasmaddin.Host()
	if !host.IsUndefined() && !host.Get("onChannel").IsUndefined() {
		d.cb = js.FuncOf(d.onReport)
		host.Call("onChannel", "webhid:report", d.cb)
	}
	return d, nil
}

type webHIDDevice struct {
	mu     sync.Mutex
	closed bool
	ch     chan MotionSample
	state  MotionSample
	last   time.Time
	cb     js.Func
}

func (d *webHIDDevice) Samples() <-chan MotionSample { return d.ch }

func (d *webHIDDevice) Close() error {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return nil
	}
	d.closed = true
	close(d.ch)
	if !d.cb.Value.IsUndefined() {
		d.cb.Release()
	}
	return nil
}

func (d *webHIDDevice) onReport(_ js.Value, args []js.Value) any {
	if len(args) == 0 || args[0].IsUndefined() || args[0].IsNull() {
		return nil
	}
	arr := args[0].Get("report")
	if arr.IsUndefined() || arr.IsNull() {
		return nil
	}
	n := arr.Length()
	if n <= 0 {
		return nil
	}
	report := make([]byte, n)
	for i := range n {
		report[i] = byte(arr.Index(i).Int())
	}
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed || !decodeInto(&d.state, report) {
		return nil
	}
	now := time.Now()
	sample := d.state
	if !d.last.IsZero() {
		sample.Period = now.Sub(d.last)
	}
	d.last = now
	select {
	case d.ch <- sample:
	default:
	}
	return nil
}
