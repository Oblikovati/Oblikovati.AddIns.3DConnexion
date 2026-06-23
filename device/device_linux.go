// SPDX-License-Identifier: GPL-2.0-only

//go:build linux && !nodevice

package device

import (
	"bufio"
	"errors"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"
)

// The Linux reader: read the SpaceMouse straight from its hidraw character device. This
// needs no 3Dconnexion driver and no X11/Wayland session (the legacy xdrvlib SDK is X11-
// only and needs a window we do not own), and the raw HID reports decode with the shared
// hidreport.go. Non-root access needs a udev rule (shipped in deploy/).
//
// UNVERIFIED: exercised only on hardware. It is defensive — a missing device or a read
// error ends the stream cleanly rather than crashing the host. (The USB-identity matching
// it relies on, isSpaceMouseID, is pure and unit-tested in vendor_test.go.)

const (
	readBufferSize = 64
	readTimeout    = 200 * time.Millisecond
	eagainBackoff  = 5 * time.Millisecond
)

var errNoDevice = errors.New("spacemouse: no 3Dconnexion hidraw device found")

type linuxDevice struct {
	f    *os.File
	ch   chan MotionSample
	done chan struct{}
}

// Open finds a 3Dconnexion hidraw node, opens it non-blocking (so the read loop can honour
// Close via a deadline), and starts streaming motion.
func Open() (Device, error) {
	path, err := findSpaceMouse()
	if err != nil {
		return nil, err
	}
	f, err := os.OpenFile(path, os.O_RDONLY|syscall.O_NONBLOCK, 0)
	if err != nil {
		return nil, err
	}
	d := &linuxDevice{f: f, ch: make(chan MotionSample, 8), done: make(chan struct{})}
	go d.read()
	return d, nil
}

func (d *linuxDevice) Samples() <-chan MotionSample { return d.ch }

// Close signals the read loop to stop and closes the device. Idempotent.
func (d *linuxDevice) Close() error {
	select {
	case <-d.done:
	default:
		close(d.done)
	}
	return d.f.Close()
}

// read decodes HID reports into a running 6-axis state and emits it on each recognised
// report, stamping the time since the last emit so the camera math is frame-rate
// independent. It exits on Close or any non-timeout read error (device unplugged).
func (d *linuxDevice) read() {
	defer close(d.ch)
	buf := make([]byte, readBufferSize)
	var cur MotionSample
	last := time.Now()
	for {
		select {
		case <-d.done:
			return
		default:
		}
		_ = d.f.SetReadDeadline(time.Now().Add(readTimeout))
		n, err := d.f.Read(buf)
		if err != nil {
			if os.IsTimeout(err) {
				continue
			}
			if errors.Is(err, syscall.EAGAIN) { // no poller: avoid a busy spin
				time.Sleep(eagainBackoff)
				continue
			}
			return
		}
		if !decodeInto(&cur, buf[:n]) {
			continue
		}
		now := time.Now()
		cur.Period = now.Sub(last)
		last = now
		select {
		case d.ch <- cur:
		case <-d.done:
			return
		}
	}
}

// findSpaceMouse returns the first /dev/hidraw* node whose HID vendor id is a SpaceMouse.
func findSpaceMouse() (string, error) {
	nodes, _ := filepath.Glob("/dev/hidraw*")
	for _, node := range nodes {
		if isSpaceMouse(node) {
			return node, nil
		}
	}
	return "", errNoDevice
}

// isSpaceMouse reads the hidraw node's sysfs uevent (HID_ID=bus:vendor:product) and reports
// whether the vendor is 3Dconnexion. Using sysfs avoids an ioctl, keeping the reader
// dependency-free.
func isSpaceMouse(node string) bool {
	uevent := filepath.Join("/sys/class/hidraw", filepath.Base(node), "device", "uevent")
	f, err := os.Open(uevent) //nolint:gosec // a fixed sysfs path derived from the node name
	if err != nil {
		return false
	}
	defer f.Close()
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		if id, ok := strings.CutPrefix(sc.Text(), "HID_ID="); ok {
			return isSpaceMouseID(id)
		}
	}
	return false
}
