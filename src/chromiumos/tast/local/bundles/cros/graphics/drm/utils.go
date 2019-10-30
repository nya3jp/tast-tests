// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package drm provides common code for interacting with the DRM subsystem.
// https://dri.freedesktop.org/docs/drm/gpu/drm-uapi.html
package drm

import (
	"os"
	"unsafe"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/input"
)

const driCardPath0 = "/dev/dri/card0"
const driCardPath1 = "/dev/dri/card1"

// isDRMAtomicSupportedForPath figures out if driCardPath supports or not DRM
// atomic, by running a given ioctl() on it.
func isDRMAtomicSupportedForPath(driCardPath string) (bool, error) {
	f, err := os.Open(driCardPath)
	if err != nil {
		return false, errors.Wrapf(err, "failed to open file %s", driCardPath)
	}
	defer f.Close()

	// Constants and structures are extracted from include/uapi/drm/drm.h
	const drmClientCapAtomic = 3
	type drmSetClientCap struct {
		capability uint64
		value      uint64
	}
	drmPack := drmSetClientCap{capability: drmClientCapAtomic, value: 1}
	const drmIoctlBase = 'd'
	const drmIoctlSetClientCap = 0x0d

	if err := input.Ioctl(int(f.Fd()), input.Iow(drmIoctlBase, drmIoctlSetClientCap, unsafe.Sizeof(drmPack)), uintptr(unsafe.Pointer(&drmPack))); err != nil {
		return false, errors.Wrapf(err, "failed to run ioctl on %s", driCardPath)
	}

	return true, nil
}

// IsDRMAtomicSupported queries the DRM API for Atomic support. If any succeeds
// it returns true.
func IsDRMAtomicSupported() (bool, error) {
	supported, err := isDRMAtomicSupportedForPath(driCardPath0)
	if err == nil {
		return supported, err
	}
	supported, err = isDRMAtomicSupportedForPath(driCardPath1)
	return supported, err
}
