// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"context"
	"encoding/binary"
	"os"
	"unsafe"

	"chromiumos/tast/errors"
)

// switchState describes the state of a binary switch reported by an EV_SW device.
type switchState int

const (
	switchOn switchState = iota
	switchOff
	switchNotFound
)

// querySwitch looks for an input device capable of reporting the switch state identified
// by ec (e.g. SW_LID or SW_TABLET_MODE) and queries and returns the current state.
func querySwitch(ctx context.Context, ec EventCode) (switchState, error) {
	infos, err := readDevices("")
	if err != nil {
		return switchNotFound, errors.Wrap(err, "failed to read devices")
	}
	var path string
	for _, info := range infos {
		if info.hasBit(evGroup, uint16(EV_SW)) && info.hasBit(switchGroup, uint16(ec)) {
			path = info.path
			break
		}
	}
	if path == "" {
		return switchNotFound, nil
	}

	f, err := os.Open(path)
	if err != nil {
		return switchNotFound, err
	}
	defer f.Close()

	// Internally, the kernel uses an array of longs and doesn't specify how endianness is handled.
	// Chrome OS is little-endian-only and SW_MAX is unlikely to reach 32 anytime soon, so we just
	// use a uint32 here to simplify the code (and since we don't currently have any way to test
	// that big-endian or >uint32 code works even if it's added).
	if kernelByteOrder != binary.LittleEndian {
		return switchNotFound, errors.New("non-little-endian unsupported")
	}
	if SW_MAX >= 32 {
		return switchNotFound, errors.New("large switch values unsupported")
	}
	var b uint32
	// This corresponds to the EVIOCGSW macro in input.h.
	if err := ioctl(int(f.Fd()), ior('E', 0x1b, unsafe.Sizeof(b)), uintptr(unsafe.Pointer(&b))); err != nil {
		return switchNotFound, err
	}

	if b&(1<<uint(ec)) != 0 {
		return switchOn, nil
	}
	return switchOff, nil
}
