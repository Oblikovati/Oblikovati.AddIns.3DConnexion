// SPDX-License-Identifier: GPL-2.0-only

//go:build darwin && !nodevice

package device

/*
#cgo LDFLAGS: -ldl
#include <dlfcn.h>
#include <stdint.h>
#include <stdbool.h>
#include <stddef.h>

// Minimal slice of the 3Dconnexion ConnexionClientAPI, declared here so the build needs
// neither the SDK header nor the framework installed: the framework is loaded at RUNTIME
// with dlopen, so a bare CI runner compiles this and a host without the driver just gets
// "no device". The struct/enum values mirror ConnexionClientAPI.h.
typedef struct ConnexionDeviceState {
  uint16_t version, client, command;
  int16_t  param;
  int32_t  value;
  uint64_t time;
  uint8_t  report[8];
  uint16_t buttons8;
  int16_t  axis[6];   // tx, ty, tz, rx, ry, rz
  uint16_t address;
  uint32_t buttons;
} ConnexionDeviceState;

enum { kConnexionMsgDeviceState = 0x33645352 };
enum { kConnexionCmdHandleButtons = 1, kConnexionCmdHandleAxis = 2 };
enum { kConnexionClientModeTakeOver = 1, kConnexionMaskAll = 0x3FFF };

typedef void     (*MsgHandlerFn)(unsigned int, unsigned int, void *);
typedef uint16_t (*RegisterFn)(uint32_t, uint8_t *, uint16_t, uint32_t);
typedef void     (*SetHandlersFn)(MsgHandlerFn, void *, void *, bool);
typedef void     (*CleanupFn)(void);
typedef void     (*UnregisterFn)(uint16_t);

static RegisterFn    sm_register;
static SetHandlersFn sm_set_handlers;
static CleanupFn     sm_cleanup;
static UnregisterFn  sm_unregister;

// Defined by cgo from the //export in device_darwin_cb.go.
extern void goSpaceMouseSample(int16_t, int16_t, int16_t, int16_t, int16_t, int16_t, uint32_t);

// The framework's message callback (runs on the framework's own thread): forward the
// 6-axis state and buttons to Go.
static void sm_message(unsigned int productID, unsigned int messageType, void *arg) {
  (void)productID;
  if (messageType != kConnexionMsgDeviceState || arg == NULL) return;
  ConnexionDeviceState *s = (ConnexionDeviceState *)arg;
  if (s->command == kConnexionCmdHandleAxis || s->command == kConnexionCmdHandleButtons) {
    goSpaceMouseSample(s->axis[0], s->axis[1], s->axis[2],
                       s->axis[3], s->axis[4], s->axis[5], s->buttons);
  }
}

// sm_open loads the installed driver framework and wires the handlers on its own thread.
// Returns 0 on success, non-zero when the framework or a required symbol is missing.
static int sm_open(void) {
  void *h = dlopen("/Library/Frameworks/3DconnexionClient.framework/3DconnexionClient", RTLD_LAZY);
  if (!h) return -1;
  sm_register     = (RegisterFn)dlsym(h, "RegisterConnexionClient");
  sm_set_handlers = (SetHandlersFn)dlsym(h, "SetConnexionHandlers");
  sm_cleanup      = (CleanupFn)dlsym(h, "CleanupConnexionHandlers");
  sm_unregister   = (UnregisterFn)dlsym(h, "UnregisterConnexionClient");
  if (!sm_register || !sm_set_handlers || !sm_cleanup) return -2;
  sm_set_handlers(sm_message, NULL, NULL, true); // true: deliver on a private thread
  return 0;
}

static uint16_t sm_register_client(void) {
  return sm_register(0, NULL, kConnexionClientModeTakeOver, kConnexionMaskAll); // 0 = wildcard
}

static void sm_close(uint16_t client) {
  if (sm_unregister && client) sm_unregister(client);
  if (sm_cleanup) sm_cleanup();
}
*/
import "C"

import (
	"errors"
	"sync"
	"time"
)

// The macOS reader: the system 3DconnexionClient framework already delivers calibrated
// 6-DOF state (ConnexionDeviceState.axis[6]), so the reader registers a handler and feeds
// each state into the shared MotionSample stream. The framework is loaded with dlopen at
// runtime (see the C preamble), so the build needs nothing installed and a host without
// the 3Dconnexion driver degrades to "no device".
//
// UNVERIFIED: exercised only on hardware. Defensive — a missing framework returns an error
// and the add-in stays loaded but inert.

var errNoFramework = errors.New("spacemouse: 3DconnexionClient framework not available")

type darwinDevice struct {
	client C.uint16_t
	ch     chan MotionSample

	mu     sync.Mutex
	last   time.Time
	closed bool
}

// active is the open device the C callback feeds. Only one client per process; guarded by
// activeMu so Open/Close and the callback never race.
var (
	activeMu sync.Mutex
	active   *darwinDevice
)

// Open loads the framework, registers a client, and starts streaming motion.
func Open() (Device, error) {
	if C.sm_open() != 0 {
		return nil, errNoFramework
	}
	d := &darwinDevice{ch: make(chan MotionSample, 8), last: time.Now()}
	activeMu.Lock()
	active = d
	activeMu.Unlock()

	d.client = C.sm_register_client()
	if d.client == 0 {
		_ = d.Close()
		return nil, errNoFramework
	}
	return d, nil
}

func (d *darwinDevice) Samples() <-chan MotionSample { return d.ch }

// Close unregisters the client and closes the stream. Idempotent.
func (d *darwinDevice) Close() error {
	activeMu.Lock()
	if active == d {
		active = nil
	}
	activeMu.Unlock()

	C.sm_close(d.client)

	d.mu.Lock()
	defer d.mu.Unlock()
	if !d.closed {
		d.closed = true
		close(d.ch)
	}
	return nil
}

// emit stamps the inter-sample period and delivers the sample, dropping it if the consumer
// is behind. Serialized with Close on d.mu so it never sends on a closed channel.
func (d *darwinDevice) emit(s MotionSample) {
	d.mu.Lock()
	defer d.mu.Unlock()
	if d.closed {
		return
	}
	now := time.Now()
	s.Period = now.Sub(d.last)
	d.last = now
	select {
	case d.ch <- s:
	default: // consumer behind: drop, the next sample carries the live state
	}
}
