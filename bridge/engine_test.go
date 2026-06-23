// SPDX-License-Identifier: GPL-2.0-only

package bridge

import (
	"testing"

	"oblikovati.org/spacemouse/device"
)

// fakeHost records the method calls the engine makes so tests can assert the camera
// frames pushed to the host without a live session.
type fakeHost struct {
	calls []hostCall
	reply map[string][]byte
}

type hostCall struct {
	method string
	req    []byte
}

func (h *fakeHost) Call(method string, req []byte) ([]byte, error) {
	h.calls = append(h.calls, hostCall{method, append([]byte(nil), req...)})
	if r, ok := h.reply[method]; ok {
		return r, nil
	}
	return []byte("{}"), nil
}

// fakeDevice is a SpaceMouse stand-in: it replays a fixed list of samples then closes.
type fakeDevice struct {
	ch     chan device.MotionSample
	closed bool
}

func newFakeDevice(samples ...device.MotionSample) *fakeDevice {
	ch := make(chan device.MotionSample, len(samples))
	for _, s := range samples {
		ch <- s
	}
	close(ch)
	return &fakeDevice{ch: ch}
}

func (d *fakeDevice) Samples() <-chan device.MotionSample { return d.ch }
func (d *fakeDevice) Close() error                        { d.closed = true; return nil }

func TestNewEngineSetupStop(t *testing.T) {
	host := &fakeHost{}
	eng := NewEngine(host, func() (device.Device, error) { return newFakeDevice(), nil })
	if eng == nil {
		t.Fatal("NewEngine returned nil")
	}
	if err := eng.Setup(); err != nil {
		t.Fatalf("Setup: %v", err)
	}
	eng.Stop() // must not panic when nothing is running
	eng.Stop() // idempotent
}
