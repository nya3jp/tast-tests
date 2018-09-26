// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"bufio"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"regexp"
	"strconv"
	"strings"
)

const (
	procDevices = "/proc/bus/input/devices" // file describing input devices
	deviceDir   = "/dev/input"              // directory containing event devices

	evGroup  = "EV"  // event type group in devInfo.bits
	keyGroup = "KEY" // keyboard event code group in devInfo.bits
)

var infoRegexp, nameRegexp, handlersRegexp, bitsRegexp, eventRegexp *regexp.Regexp

func init() {
	// These match lines in /proc/bus/input/devices. See readDevices for details.
	infoRegexp = regexp.MustCompile(`^I: Bus=([0-9a-f]{4}) Vendor=([0-9a-f]{4}) Product=([0-9a-f]{4}) Version=([0-9a-f]{4})$`)
	nameRegexp = regexp.MustCompile(`^N: Name="(.+)"$`)
	handlersRegexp = regexp.MustCompile(`^H: Handlers=(.+)$`)
	bitsRegexp = regexp.MustCompile(`^B: ([A-Z]+)=([0-9a-f ]+)$`)

	// This matches an event device name within deviceDir.
	eventRegexp = regexp.MustCompile(`^event\d+`)
}

// devInfo contains information about a device.
type devInfo struct {
	name string // descriptive name, e.g. "AT Translated Set 2 keyboard"
	path string // path to event device, e.g. "/dev/input/event3"

	bits map[string]*big.Int // bitfields keyed by group name, e.g. "EV" or "KEY"

	bus, vendor, product, version uint16 // misc info
}

func newDevInfo() *devInfo {
	return &devInfo{bits: make(map[string]*big.Int)}
}

func (di *devInfo) String() string {
	return fmt.Sprintf("%s (%q, bus %04x, ID %04x:%04x)", di.path, di.name, di.bus, di.vendor, di.product)
}

// isKeyboard returns true if this appears to be a keyboard device.
func (di *devInfo) isKeyboard() bool {
	// Just check some arbitrary keys. The choice of 1, Q, and Space comes from
	// client/cros/input_playback/input_playback.py in the Autotest repo.
	return di.path != "" && di.hasBit(evGroup, uint16(EV_KEY)) &&
		di.hasBit(keyGroup, uint16(KEY_1)) && di.hasBit(keyGroup, uint16(KEY_Q)) && di.hasBit(keyGroup, uint16(KEY_SPACE))
}

// hasBit returns true if the n-th bit in di.bits is set.
func (di *devInfo) hasBit(grp string, n uint16) bool {
	bits, ok := di.bits[grp]
	return ok && bits.Bit(int(n)) != 0
}

// parseLine parses a single line from a devices file and incorporates it into di.
// See readDevices for information about the expected format.
func (di *devInfo) parseLine(line string) error {
	if ms := infoRegexp.FindStringSubmatch(line); ms != nil {
		id := func(s string) uint16 {
			n, _ := strconv.ParseUint(s, 16, 16)
			return uint16(n)
		}
		di.bus, di.vendor, di.product, di.version = id(ms[1]), id(ms[2]), id(ms[3]), id(ms[4])
	} else if ms = nameRegexp.FindStringSubmatch(line); ms != nil {
		di.name = ms[1]
	} else if ms = handlersRegexp.FindStringSubmatch(line); ms != nil {
		for _, h := range strings.Fields(ms[1]) {
			if eventRegexp.MatchString(h) {
				di.path = filepath.Join(deviceDir, h)
				break
			}
		}
	} else if ms = bitsRegexp.FindStringSubmatch(line); ms != nil {
		var str string
		// Bitfields are specified as space-separated 64-bit hex values.
		// Zero-pad if necessary.
		for _, p := range strings.Fields(ms[2]) {
			if len(p) < 16 {
				p = strings.Repeat("0", 16-len(p)) + p
			}
			str += p
		}
		bits, ok := big.NewInt(0).SetString(str, 16)
		if !ok {
			return fmt.Errorf("failed to parse bitfield %q", str)
		}
		di.bits[ms[1]] = bits
	}
	return nil
}

// readDevices reads a file describing devices (typically /proc/bus/input/devices)
// and returns device information.
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
func readDevices(path string) (infos []*devInfo, err error) {
	f, err := os.Open(path)
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
		infos[len(infos)-1].parseLine(line)
	}
	if err := sc.Err(); err != nil {
		return nil, err
	}
	return infos, nil
}
