// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"encoding/binary"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"strings"
	"unsafe"

	"chromiumos/tast/errors"
)

const (
	uinputDev    = "/dev/uinput"
	sysfsVirtDir = "/sys/devices/virtual/input"

	// These constants are defined in include/uapi/linux/uinput.h.
	uinputIoctlBase  = 'U' // UINPUT_IOCTL_BASE
	devCreateIoctl   = 1   // UI_DEV_CREATE
	devSetupIoctl    = 3   // UI_DEV_SETUP
	absSetupIoctl    = 4   // UI_ABS_SETUP
	getSysnameIoctl  = 44  // UI_GET_SYSNAME
	setEvbitIoctl    = 100 // UI_SET_EVBIT
	setPropbitIoctl  = 110 // UI_SET_PROPBIT
	uinputMaxNameLen = 80  // UINPUT_MAX_NAME_SIZE
)

// Values are ioctl offsets (see the nr arg to ior and iow) from include/uapi/linux/uinput.h.
var eventTypeIoctls = map[EventType]uint{
	EV_KEY: 101, // UI_SET_KEYBIT
	EV_REL: 102, // UI_SET_RELBIT
	EV_ABS: 103, // UI_SET_ABSBIT
	EV_MSC: 104, // UI_SET_MSCBIT
	EV_LED: 105, // UI_SET_LEDBIT
	EV_SND: 106, // UI_SET_SNDBIT
	EV_SW:  109, // UI_SET_SWBIT
}

// createVirtual creates a virtual input device using the Linux kernel's uinput module.
//
// name is a human-readable name for the device with a maximum length of 80 bytes.
// id contains additional information used to identify the device.
// props contains the device's properties (corresponding to the PROP bitfield).
// eventTypes contains supported event types (corresponding to the EV bitfield).
// eventCodes contains supported event codes for each event type (see e.g. the KEY and REL bitfields).
// The returned path contains the device node, e.g. "/dev/input/event4".
// f must be held open while using the device and should be closed to destroy the device.
//
// If multiple devices are created simultaneously, it's necessary to give them unique name/id combinations,
// as these values are used to identify the sysfs node on pre-v3.14 kernels.
//
// This function is similar to calling libevdev_uinput_create_from_device() with LIBEVDEV_UINPUT_OPEN_MANAGED.
func createVirtual(name string, id devID, props, eventTypes uint32,
	eventCodes map[EventType]*big.Int, axes map[AxisCode]AxisInfo) (path string, f *os.File, err error) {
	if len(name) > uinputMaxNameLen {
		return "", nil, errors.Errorf("name %q exceeds %d-byte limit", name, uinputMaxNameLen)
	}

	// ioctls are made on /dev/uinput to describe how the virtual device should be created.
	// The device will exist as long as this file is held open.
	f, err = os.OpenFile(uinputDev, os.O_RDWR, 0644)
	if err != nil {
		return "", nil, err
	}
	fd := int(f.Fd())

	// Close the uinput file if we encounter an error before completing device creation.
	fileToClose := f
	defer func() {
		if fileToClose != nil {
			fileToClose.Close()
		}
	}()

	// Make a UI_SET_EVBIT ioctl for each supported event type.
	for i := uint32(0); i < 32; i++ {
		if (eventTypes>>i)&0x1 == 0 {
			continue
		}
		if err := ioctl(fd, iow(uinputIoctlBase, setEvbitIoctl, unsafe.Sizeof(i)), uintptr(i)); err != nil {
			return "", nil, errors.Wrapf(err, "failed setting EV bit %#x", i)
		}
	}

	// Make a UI_SET_PROPBIT ioctl for each supported property.
	for i := uint32(0); i < 32; i++ {
		if (props>>i)&0x1 == 0 {
			continue
		}
		if err := ioctl(fd, iow(uinputIoctlBase, setPropbitIoctl, unsafe.Sizeof(i)), uintptr(i)); err != nil {
			return "", nil, errors.Wrapf(err, "failed setting PROP bit %#x", i)
		}
	}

	// Make a UI_SET_<type>BIT ioctl for each supported (event type, event code) pair.
	for et, ecs := range eventCodes {
		etIoctl, ok := eventTypeIoctls[et]
		if !ok {
			return "", f, errors.Errorf("unsupported event type %#v", et)
		}
		for ec := uint32(0); int(ec) < ecs.BitLen(); ec++ {
			if ecs.Bit(int(ec)) == 0 {
				continue
			}
			if err := ioctl(fd, iow(uinputIoctlBase, etIoctl, unsafe.Sizeof(ec)), uintptr(ec)); err != nil {
				return "", nil, errors.Wrapf(err, "failed setting event code %#x for event type %#x", ec, et)
			}
		}
	}

	// Set the device's name and ID.
	if err := performVirtDevSetup(f, name, id, axes); err != nil {
		return "", nil, errors.Wrap(err, "failed setting up device")
	}

	// Make a UI_DEV_CREATE ioctl to finalize creation of the device.
	if err := ioctl(fd, ioc(iocNone, uinputIoctlBase, devCreateIoctl, 0), uintptr(0)); err != nil {
		return "", nil, errors.Wrap(err, "UI_DEV_CREATE ioctl failed")
	}

	// Find the device's sysfs dir and then use it to find the device's path in /dev.
	if sysdir, err := getVirtDevSysfsPath(fd, name, id); err != nil {
		return "", nil, errors.Wrap(err, "didn't find sysfs dir")
	} else if path, err = getDevicePath(sysdir, ""); err != nil {
		return "", nil, errors.Wrap(err, "didn't find device")
	}

	fileToClose = nil // disarm cleanup
	return path, f, nil
}

// performVirtDevSetup makes a UI_DEV_SETUP ioctl to a uinput FD to configure a virtual device.
func performVirtDevSetup(f *os.File, name string, id devID, axes map[AxisCode]AxisInfo) error {
	// Try writing a uinput_setup struct via the ioctl first.
	uinputSetup := struct {
		id           devID
		name         [uinputMaxNameLen]byte
		ffEffectsMax uint32
	}{id: id}
	copy(uinputSetup.name[:], []byte(name))

	if err := ioctl(int(f.Fd()), iow(uinputIoctlBase, devSetupIoctl, unsafe.Sizeof(uinputSetup)),
		uintptr(unsafe.Pointer(&uinputSetup))); err == nil {
		if axes != nil {
			return performVirtDevAxesSetup(f, axes)
		}
		return nil
	}

	// UI_DEV_SETUP is only available in v3.14 and newer kernels.
	// If the ioctl failed, fall back to the old method of writing a uinput_user_dev struct directly to uinput.
	const absCnt = 0x40 // ABS_CNT, i.e. ABS_MAX+1
	uinputUserDev := struct {
		name                             [uinputMaxNameLen]byte
		id                               devID
		ffEffectsMax                     uint32
		absMax, absMin, absFuzz, absFlat [absCnt]int32
	}{id: id}
	copy(uinputUserDev.name[:], []byte(name))

	if axes != nil {
		for code, info := range axes {
			uinputUserDev.absMax[code] = info.Maximum
			uinputUserDev.absMin[code] = info.Minimum
			uinputUserDev.absFuzz[code] = info.Fuzz
			uinputUserDev.absFlat[code] = info.Flat
		}
	}

	if err := binary.Write(f, binary.LittleEndian, &uinputUserDev); err != nil {
		return errors.Wrap(err, "UI_DEV_SETUP ioctl and old-style write both failed")
	}
	return nil
}

func performVirtDevAxesSetup(f *os.File, axes map[AxisCode]AxisInfo) error {
	for code, info := range axes {
		uinputAbsSetup := struct {
			code       uint16
			value      int32
			minimum    int32
			maximum    int32
			fuzz       int32
			flat       int32
			resolution int32
		}{
			code:       uint16(code),
			value:      0,
			minimum:    info.Minimum,
			maximum:    info.Maximum,
			fuzz:       info.Fuzz,
			flat:       info.Flat,
			resolution: info.Resolution}
		if err := ioctl(int(f.Fd()), iow(uinputIoctlBase, absSetupIoctl, unsafe.Sizeof(uinputAbsSetup)),
			uintptr(unsafe.Pointer(&uinputAbsSetup))); err != nil {
			return err
		}
	}
	return nil
}

// getVirtDevSysfsPath makes a UI_GET_SYSNAME ioctl to a uinput FD to find a virtual device's sysfs path.
func getVirtDevSysfsPath(fd int, name string, id devID) (string, error) {
	// Try the ioctl first.
	var buf [64]byte
	if err := ioctl(fd, ior(uinputIoctlBase, getSysnameIoctl, uintptr(len(buf))),
		uintptr(unsafe.Pointer(&buf))); err == nil {
		sysname := strings.TrimRight(string(buf[:]), "\x00") // trim trailing NULs
		return filepath.Join(sysfsVirtDir, sysname), nil
	}

	// UI_GET_SYSNAME is only available in v3.14 and newer kernels.
	// If the ioctl failed, iterate over all virtual devices to find the one with the name and ID that we used.
	fis, err := ioutil.ReadDir(sysfsVirtDir)
	if err != nil {
		return "", errors.Wrap(err, "UI_DEV_SETUP ioctl failed and no virtual devices found")
	}
	for _, fi := range fis {
		dir := filepath.Join(sysfsVirtDir, fi.Name())
		sysfsName, err := ioutil.ReadFile(filepath.Join(dir, "name"))
		if err != nil || strings.TrimSpace(string(sysfsName)) != name {
			continue
		}

		checkID := func(name string, val uint16) bool {
			b, err := ioutil.ReadFile(filepath.Join(dir, "id", name))
			return err == nil && strings.TrimSpace(string(b)) == fmt.Sprintf("%04x", val)
		}
		if checkID("bustype", id.bustype) && checkID("vendor", id.vendor) &&
			checkID("product", id.product) && checkID("version", id.version) {
			return dir, nil
		}
	}

	return "", errors.Errorf("UI_DEV_SETUP ioctl failed and device not found in %v", sysfsVirtDir)
}

// makeBigInt is a convenience function that takes a slice of 64-bit bitfields (as seen in /proc/bus/input/devices)
// and combines them into a big.Int value. The most-significant bitfield appears first.
func makeBigInt(nums []uint64) *big.Int {
	bits := big.NewInt(0)
	for _, num := range nums {
		bits.Lsh(bits, 64).Or(bits, big.NewInt(0).SetUint64(num))
	}
	return bits
}
