// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"

	"chromiumos/tast/errors"
)

const (
	procDevices = "/proc/bus/input/devices" // file describing input devices
	sysfsDir    = "/sys"                    // base sysfs directory
	deviceDir   = "/dev/input"              // directory containing event devices

	evGroup     = "EV"  // event type group in devInfo.bits
	keyGroup    = "KEY" // keyboard event code group in devInfo.bits
	switchGroup = "SW"  // switch event code group in devInfo.bits
	absGroup    = "ABS" // absolute type group in devInfo.bits
)

// These match lines in /proc/bus/input/devices. See readDevices for details.
var infoRegexp = regexp.MustCompile(`^I: Bus=([0-9a-f]{4}) Vendor=([0-9a-f]{4}) Product=([0-9a-f]{4}) Version=([0-9a-f]{4})$`)
var nameRegexp = regexp.MustCompile(`^N: Name="(.+)"$`)
var physRegexp = regexp.MustCompile(`^P: Phys=(.+)$`)
var sysfsRegexp = regexp.MustCompile(`^S: Sysfs=(.+)$`)
var bitsRegexp = regexp.MustCompile(`^B: ([A-Z]+)=([0-9a-f ]+)$`)

// devInfo contains information about a device.
type devInfo struct {
	name string // descriptive name, e.g. "AT Translated Set 2 keyboard"
	phys string // physical path, e.g. "isa0060/serio0/input0"
	path string // path to event device, e.g. "/dev/input/event3"

	bits map[string]*big.Int // bitfields keyed by group name, e.g. "EV" or "KEY"

	devID
}

// devID contains information identifying a device.
// Do not change the type or order of fields, as this is used in various kernel structs.
type devID struct{ bustype, vendor, product, version uint16 }

func newDevInfo() *devInfo {
	return &devInfo{bits: make(map[string]*big.Int)}
}

func (di *devInfo) String() string {
	return fmt.Sprintf("%s (%q %04x:%04x:%04x:%04x)",
		di.path, di.name, di.bustype, di.vendor, di.product, di.version)
}

// isKeyboard returns true if this appears to be a keyboard device.
func (di *devInfo) isKeyboard() bool {
	// Just check some arbitrary keys. The choice of 1, Q, and Space comes from
	// client/cros/input_playback/input_playback.py in the Autotest repo.
	return di.path != "" && di.hasBit(evGroup, uint16(EV_KEY)) &&
		di.hasBit(keyGroup, uint16(KEY_1)) && di.hasBit(keyGroup, uint16(KEY_Q)) && di.hasBit(keyGroup, uint16(KEY_SPACE))
}

// isTouchscreen returns true if this appears to be a touchscreen device.
func (di *devInfo) isTouchscreen() bool {
	// Touchscreen reports values in absolute coordinates, and should have the BTN_TOUCH bit set.
	// Multitouch (bit ABS_MT_SLOT) is required to differentiate itself from some stylus devices.
	// Some touchpad devices (like in Kevin) implement all the features needed for a touchscreen
	// device, and luckily more. So, to differentiate a touchpad from a touchscreen, we filter out
	// devices that implements features like DOUBLETAP, which should not be present on a touchscreen.
	return di.path != "" &&
		di.hasBit(evGroup, uint16(EV_KEY)) &&
		di.hasBit(evGroup, uint16(EV_ABS)) &&
		di.hasBit(keyGroup, uint16(BTN_TOUCH)) &&
		!di.hasBit(keyGroup, uint16(BTN_TOOL_DOUBLETAP)) &&
		di.hasBit(absGroup, uint16(ABS_MT_SLOT))
}

// hasBit returns true if the n-th bit in di.bits is set.
func (di *devInfo) hasBit(grp string, n uint16) bool {
	bits, ok := di.bits[grp]
	return ok && bits.Bit(int(n)) != 0
}

// parseLine parses a single line from a devices file and incorporates it into di.
// See readDevices for information about the expected format.
func (di *devInfo) parseLine(line, root string) error {
	if ms := infoRegexp.FindStringSubmatch(line); ms != nil {
		id := func(s string) uint16 {
			n, _ := strconv.ParseUint(s, 16, 16)
			return uint16(n)
		}
		di.bustype, di.vendor, di.product, di.version = id(ms[1]), id(ms[2]), id(ms[3]), id(ms[4])
	} else if ms = nameRegexp.FindStringSubmatch(line); ms != nil {
		di.name = ms[1]
	} else if ms = physRegexp.FindStringSubmatch(line); ms != nil {
		di.phys = ms[1]
	} else if ms = sysfsRegexp.FindStringSubmatch(line); ms != nil {
		var err error
		dir := filepath.Join(sysfsDir, ms[1])
		if di.path, err = getDevicePath(dir, root); err != nil {
			return errors.Wrapf(err, "didn't find device in %v", dir)
		}
	} else if ms = bitsRegexp.FindStringSubmatch(line); ms != nil {
		var str string
		// Bitfields are specified as space-separated 32- or 64-bit hex values
		// (depending on the userspace arch). Zero-pad if necessary.
		ptrSize := 32 << uintptr(^uintptr(0)>>63) // from https://stackoverflow.com/questions/25741841/
		fullLen := ptrSize / 4
		for _, p := range strings.Fields(ms[2]) {
			if len(p) < fullLen {
				p = strings.Repeat("0", fullLen-len(p)) + p
			}
			str += p
		}
		bits, ok := big.NewInt(0).SetString(str, 16)
		if !ok {
			return errors.Errorf("failed to parse bitfield %q", str)
		}
		di.bits[ms[1]] = bits
	}
	return nil
}

// readDevices reads /proc/bus/input/devices and returns device information.
// Unit tests may specify an alternate root directory via root.
//
// The file should contain stanzas similar to the following, separated by blank lines:
//
//	I: Bus=0011 Vendor=0001 Product=0001 Version=ab83
//	N: Name="AT Translated Set 2 keyboard"
//	P: Phys=isa0060/serio0/input0
//	S: Sysfs=/devices/platform/i8042/serio0/input/input3
//	U: Uniq=
//	H: Handlers=sysrq event3
//	B: PROP=0
//	B: EV=120013
//	B: KEY=402000000 3803078f800d001 feffffdfffefffff fffffffffffffffe
//	B: MSC=10
//	B: LED=7
//
// "B" entries are hexadecimal bitfields. For example, in the "EV" bitfield, the i-th bit corresponds to the EventType with value i.
func readDevices(root string) (infos []*devInfo, err error) {
	f, err := os.Open(filepath.Join(root, procDevices))
	if err != nil {
		return nil, err
	}
	defer f.Close()

	inDev := false
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		line := strings.TrimSpace(sc.Text())

		// End the current device when we see a blank line.
		if len(line) == 0 {
			inDev = false
			continue
		}

		if !inDev {
			infos = append(infos, newDevInfo())
			inDev = true
		}
		if err := infos[len(infos)-1].parseLine(line, root); err != nil {
			return nil, errors.Wrapf(err, "failed parsing %q from %v", line, procDevices)
		}
	}
	if err := sc.Err(); err != nil {
		return nil, errors.Wrapf(err, "failed reading %v", procDevices)
	}
	return infos, nil
}

// getDevicePath iterates over the entries in sysdir, a sysfs device dir (e.g.
// "/sys/devices/platform/i8042/serio1/input/input3"), looking for a event dir (e.g. "event3"), and returns the
// corresponding device in /dev/input (e.g. "/dev/input/event3").
// Unit tests may specify an alternate root directory via root.
func getDevicePath(sysdir, root string) (string, error) {
	fis, err := ioutil.ReadDir(filepath.Join(root, sysdir))
	if err != nil {
		return "", err
	}
	for _, fi := range fis {
		if !strings.HasPrefix(fi.Name(), "event") || !fi.Mode().IsDir() {
			continue
		}
		dev := filepath.Join(deviceDir, fi.Name())
		if _, err := os.Stat(filepath.Join(root, dev)); err == nil {
			return dev, nil
		}
	}
	return "", errors.Errorf("no event dirs in %v", sysdir)
}
