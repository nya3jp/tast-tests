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
	IOCNone  = 0
	IOCWrite = 1
	IOCRead  = 2

	IOCNrBits   = 8
	IOCTypeBits = 8

	// TODO: Might not be portable accross different architectures.
	// See include/asm-generic/ioctl.h for further info.
	IOCSizeBits = 14
	IOCDirBits  = 2

	IOCNrMask   = ((1 << IOCNrBits) - 1)
	IOCTypeMask = ((1 << IOCTypeBits) - 1)
	IOCSizeMask = ((1 << IOCSizeBits) - 1)
	IOCDirMask  = ((1 << IOCDirBits) - 1)

	IOCNrShift   = 0
	IOCTypeShift = (IOCNrShift + IOCNrBits)
	IOCSizeShift = (IOCTypeShift + IOCTypeBits)
	IOCDirShift  = (IOCSizeShift + IOCSizeBits)
)

// IOC returns an encoded IOCTL value based on dir, type, nr and size.
func IOC(dir int, t int, nr int, size int) int {
	return (dir << IOCDirShift) | (t << IOCTypeShift) | (nr << IOCNrShift) |
		(size << IOCSizeShift)
}

// IOR returns an enconded a Read IOCLT value based on type, nr and size.
func IOR(t int, nr int, size int) int {
	return IOC(IOCRead, t, nr, size)
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
