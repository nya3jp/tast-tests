// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE

package input

import (
	"syscall"
	"unsafe"
)

// IOCTL related constants and functions taken from Linux kernel:
// include/asm-generic/ioctl.h
const (
	iocNone  = 0
	iocWrite = 1
	iocRead  = 2

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

// IOC returns an encoded IOCTL value based on dir, t, nr and size.
func IOC(dir, t, nr, size int) int {
	return (dir << iocDirShift) | (t << iocTypeShift) | (nr << iocNrShift) |
		(size << iocSizeShift)
}

// IOR returns an encoded Read IOCTL value based on t, nr and size.
func IOR(t, nr, size int) int {
	return IOC(iocRead, t, nr, size)
}

// Ioctl calls the ioctl system call.
func Ioctl(fd uintptr, name int, data unsafe.Pointer) error {
	var err error
	_, _, errno := syscall.RawSyscall(syscall.SYS_IOCTL, fd, uintptr(name), uintptr(data))
	if errno != 0 {
		err = errno
	}
	return err
}
