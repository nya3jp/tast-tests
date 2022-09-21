// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package input

import (
	"golang.org/x/sys/unix"
)

// iocDir describes the direction data will be transferred during an ioctl.
type iocDir uint

const (
	iocNone  iocDir = 0
	iocWrite        = 1
	iocRead         = 2
)

// ioctl-related constants and functions taken from Linux kernel:
// include/asm-generic/ioctl.h
const (
	iocNrBits   = 8
	iocTypeBits = 8

	// TODO: On PowerPC, SPARC, MIPS and Alpha it is defined as a 13-bit constant.
	// In the rest, including Intel and ARM it is defined as a 14-bit constant.
	// See https://elixir.bootlin.com/linux/latest/ident/_IOC_SIZEBITS
	iocSizeBits = 14
	iocDirBits  = 2

	iocNrMask   = (1 << iocNrBits) - 1
	iocTypeMask = (1 << iocTypeBits) - 1
	iocSizeMask = (1 << iocSizeBits) - 1
	iocDirMask  = (1 << iocDirBits) - 1

	iocNrShift   = 0
	iocTypeShift = iocNrShift + iocNrBits
	iocSizeShift = iocTypeShift + iocTypeBits
	iocDirShift  = iocSizeShift + iocSizeBits
)

// ioc returns an encoded ioctl request in the given direction for the supplied type
// (e.g. UINPUT_IOCTL_BASE), number (e.g. UI_DEV_CREATE), and data size.
// This is analogous to the _IOC C macro.
func ioc(dir iocDir, typ, nr uint, size uintptr) uint {
	return (uint(dir) << iocDirShift) | (typ << iocTypeShift) | (nr << iocNrShift) |
		(uint(size) << iocSizeShift)
}

// ior returns an encoded read ioctl request. See ioc for arguments.
// This is analogous to the _IOR C macro.
func ior(typ, nr uint, size uintptr) uint {
	return ioc(iocRead, typ, nr, size)
}

// iow returns an encoded write ioctl request. See ioc for arguments.
// This is analogous to the _IOW C macro.
func iow(typ, nr uint, size uintptr) uint {
	return ioc(iocWrite, typ, nr, size)
}

// ioctl makes an ioctl system call against fd using the supplied encoded request and data.
func ioctl(fd int, req uint, data uintptr) error {
	if _, _, errno := unix.RawSyscall(unix.SYS_IOCTL, uintptr(fd), uintptr(req), data); errno != 0 {
		return errno
	}
	return nil
}
