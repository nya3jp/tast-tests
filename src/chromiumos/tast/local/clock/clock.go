// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clock

import (
	"syscall"
	"time"
	"unsafe"
)

// Defined in /usr/include/linux/time.h.
type Clock int

const (
	// Monotonic represents CLOCK_MONOTONIC. It measures monotonically-increasing
	// time since an unspecified starting point. The clock does not advance while the
	// system is suspended.
	Monotonic Clock = 1
	// BootTime represents CLOCK_BOOTTIME. It is similar to Monotonic but also
	// advances while the system is suspended.
	BootTime Clock = 7
)

// Now returns c's current value, expressed as the amount of elapsed time since the
// start of the clock.
func Now(c Clock) time.Duration {
	var ts syscall.Timespec
	syscall.Syscall(syscall.SYS_CLOCK_GETTIME, uintptr(c), uintptr(unsafe.Pointer(&ts)), 0)
	return time.Duration(ts.Nano()) * time.Nanosecond
}
