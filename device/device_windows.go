// SPDX-License-Identifier: GPL-2.0-only

//go:build windows && !nodevice

package device

import (
	"errors"
	"syscall"
	"time"
	"unsafe"
)

// The Windows reader: find the SpaceMouse through the Win32 HID interface and read its raw
// input reports, which decode with the shared hidreport.go. It uses only the standard
// syscall package + lazily-loaded system DLLs (hid.dll, setupapi.dll) — no cgo and no
// extra module deps. The vendor navlib SDK is deliberately not linked: it ships an
// MSVC-only import lib and its model drives the camera in-process, which does not fit an
// add-in that moves the camera over JSON-RPC.
//
// UNVERIFIED: exercised only on hardware. It is defensive — a missing device or a read
// error ends the stream cleanly rather than crashing the host. The reader uses a blocking
// ReadFile cancelled from Close via CancelIoEx.

var (
	modhid      = syscall.NewLazyDLL("hid.dll")
	modsetupapi = syscall.NewLazyDLL("setupapi.dll")
	modkernel32 = syscall.NewLazyDLL("kernel32.dll")

	procHidDGetHidGuid                  = modhid.NewProc("HidD_GetHidGuid")
	procHidDGetAttributes               = modhid.NewProc("HidD_GetAttributes")
	procSetupDiGetClassDevs             = modsetupapi.NewProc("SetupDiGetClassDevsW")
	procSetupDiEnumDeviceInterfaces     = modsetupapi.NewProc("SetupDiEnumDeviceInterfaces")
	procSetupDiGetDeviceInterfaceDetail = modsetupapi.NewProc("SetupDiGetDeviceInterfaceDetailW")
	procSetupDiDestroyDeviceInfoList    = modsetupapi.NewProc("SetupDiDestroyDeviceInfoList")
	procCancelIoEx                      = modkernel32.NewProc("CancelIoEx")
)

const (
	digcfPresent         = 0x02
	digcfDeviceInterface = 0x10
	invalidHandle        = ^uintptr(0)
	reportBufferSize     = 64
)

// hidAttributes mirrors HIDD_ATTRIBUTES.
type hidAttributes struct {
	Size          uint32
	VendorID      uint16
	ProductID     uint16
	VersionNumber uint16
}

// spDeviceInterfaceData mirrors SP_DEVICE_INTERFACE_DATA.
type spDeviceInterfaceData struct {
	CbSize             uint32
	InterfaceClassGuid syscall.GUID
	Flags              uint32
	Reserved           uintptr
}

var errNoDevice = errors.New("spacemouse: no 3Dconnexion HID device found")

type windowsDevice struct {
	handle syscall.Handle
	ch     chan MotionSample
	done   chan struct{}
}

// Open finds a 3Dconnexion HID device, opens it, and starts streaming motion.
func Open() (Device, error) {
	handle, err := openSpaceMouse()
	if err != nil {
		return nil, err
	}
	d := &windowsDevice{handle: handle, ch: make(chan MotionSample, 8), done: make(chan struct{})}
	go d.read()
	return d, nil
}

func (d *windowsDevice) Samples() <-chan MotionSample { return d.ch }

// Close cancels the in-flight read and closes the handle. Idempotent.
func (d *windowsDevice) Close() error {
	select {
	case <-d.done:
	default:
		close(d.done)
	}
	_, _, _ = procCancelIoEx.Call(uintptr(d.handle), 0) // unblock ReadFile
	return syscall.CloseHandle(d.handle)
}

// read decodes HID reports into a running 6-axis state and emits it on each recognised
// report, stamping the time since the last emit. It exits on Close (read aborted) or any
// read error.
func (d *windowsDevice) read() {
	defer close(d.ch)
	buf := make([]byte, reportBufferSize)
	var cur MotionSample
	last := time.Now()
	for {
		select {
		case <-d.done:
			return
		default:
		}
		var n uint32
		err := syscall.ReadFile(d.handle, buf, &n, nil)
		if err != nil || n == 0 {
			return // device gone or read cancelled by Close
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

// openSpaceMouse enumerates HID device interfaces and opens the first whose attributes
// match a SpaceMouse vendor/product.
func openSpaceMouse() (syscall.Handle, error) {
	var guid syscall.GUID
	_, _, _ = procHidDGetHidGuid.Call(uintptr(unsafe.Pointer(&guid)))

	devInfo, _, _ := procSetupDiGetClassDevs.Call(
		uintptr(unsafe.Pointer(&guid)), 0, 0, digcfPresent|digcfDeviceInterface)
	if devInfo == invalidHandle || devInfo == 0 {
		return 0, errNoDevice
	}
	defer procSetupDiDestroyDeviceInfoList.Call(devInfo)

	for i := uint32(0); ; i++ {
		path, ok := interfacePath(devInfo, &guid, i)
		if !ok {
			break
		}
		if handle, ok := openIfSpaceMouse(path); ok {
			return handle, nil
		}
	}
	return 0, errNoDevice
}

// interfacePath returns the device path of the i-th HID interface, or ok=false when the
// enumeration is exhausted.
func interfacePath(devInfo uintptr, guid *syscall.GUID, index uint32) (string, bool) {
	iface := spDeviceInterfaceData{CbSize: uint32(unsafe.Sizeof(spDeviceInterfaceData{}))}
	r, _, _ := procSetupDiEnumDeviceInterfaces.Call(
		devInfo, 0, uintptr(unsafe.Pointer(guid)), uintptr(index), uintptr(unsafe.Pointer(&iface)))
	if r == 0 {
		return "", false
	}
	var needed uint32
	procSetupDiGetDeviceInterfaceDetail.Call(
		devInfo, uintptr(unsafe.Pointer(&iface)), 0, 0, uintptr(unsafe.Pointer(&needed)), 0)
	if needed == 0 {
		return "", true // skip this one but keep enumerating
	}
	// SP_DEVICE_INTERFACE_DETAIL_DATA_W: { DWORD cbSize; WCHAR DevicePath[1] }. cbSize is
	// the fixed header size (4 + 2 padding to WCHAR alignment on 64-bit = 8).
	buf := make([]byte, needed)
	detail := (*detailHeader)(unsafe.Pointer(&buf[0]))
	detail.CbSize = detailHeaderSize
	r, _, _ = procSetupDiGetDeviceInterfaceDetail.Call(
		devInfo, uintptr(unsafe.Pointer(&iface)),
		uintptr(unsafe.Pointer(&buf[0])), uintptr(needed), 0, 0)
	if r == 0 {
		return "", true
	}
	return pathFromDetail(buf), true
}

// detailHeader is the fixed head of SP_DEVICE_INTERFACE_DETAIL_DATA_W (the DevicePath WCHARs
// follow it in the same buffer).
type detailHeader struct{ CbSize uint32 }

// detailHeaderSize is the documented cbSize the API expects: 6 on 32-bit, 8 on 64-bit
// (4-byte DWORD + alignment to the following WCHAR). unsafe.Sizeof gives the platform value.
var detailHeaderSize = func() uint32 {
	if unsafe.Sizeof(uintptr(0)) == 8 {
		return 8
	}
	return 6
}()

// pathFromDetail decodes the UTF-16 device path that follows the 4-byte cbSize header.
func pathFromDetail(buf []byte) string {
	const off = 4 // the path WCHARs start right after the DWORD cbSize
	if len(buf) <= off {
		return ""
	}
	u16 := unsafe.Slice((*uint16)(unsafe.Pointer(&buf[off])), (len(buf)-off)/2)
	return syscall.UTF16ToString(u16)
}

// openIfSpaceMouse opens the device path and keeps the handle only if its HID attributes
// identify a SpaceMouse; otherwise it closes it and reports false.
func openIfSpaceMouse(path string) (syscall.Handle, bool) {
	p, err := syscall.UTF16PtrFromString(path)
	if err != nil {
		return 0, false
	}
	handle, err := syscall.CreateFile(p,
		syscall.GENERIC_READ, syscall.FILE_SHARE_READ|syscall.FILE_SHARE_WRITE,
		nil, syscall.OPEN_EXISTING, 0, 0)
	if err != nil {
		return 0, false
	}
	attrs := hidAttributes{Size: uint32(unsafe.Sizeof(hidAttributes{}))}
	r, _, _ := procHidDGetAttributes.Call(uintptr(handle), uintptr(unsafe.Pointer(&attrs)))
	if r == 0 || !isSpaceMouseVIDPID(attrs.VendorID, attrs.ProductID) {
		_ = syscall.CloseHandle(handle)
		return 0, false
	}
	return handle, true
}
