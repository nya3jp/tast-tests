// Copyright 2018 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package fscaps reads Linux file capabilities.
//
// See capabilities(7) for details.
package fscaps

import (
	"fmt"
	"strings"
	"unsafe"

	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
)

func init() {
	var err error
	if attrName, err = unix.BytePtrFromString("security.capability"); err != nil {
		panic(fmt.Sprintf("Failed to allocate extended attribute byte pointer: %v", err))
	}
}

// attrName holds a NUL-terminated "security.capability" string used to make LGETXATTR syscalls.
var attrName *byte

// These are masks rather than the bare 0, 1, 2, etc. values from the corresponding CAP_* #defines
// in linux/capability.h to make it easier to construct Caps structs.
const (
	CHOWN            = 1 << 0  // NOLINT
	DAC_OVERRIDE     = 1 << 1  // NOLINT
	DAC_READ_SEARCH  = 1 << 2  // NOLINT
	FOWNER           = 1 << 3  // NOLINT
	FSETID           = 1 << 4  // NOLINT
	KILL             = 1 << 5  // NOLINT
	SETGID           = 1 << 6  // NOLINT
	SETUID           = 1 << 7  // NOLINT
	SETPCAP          = 1 << 8  // NOLINT
	LINUX_IMMUTABLE  = 1 << 9  // NOLINT
	NET_BIND_SERVICE = 1 << 10 // NOLINT
	NET_BROADCAST    = 1 << 11 // NOLINT
	NET_ADMIN        = 1 << 12 // NOLINT
	NET_RAW          = 1 << 13 // NOLINT
	IPC_LOCK         = 1 << 14 // NOLINT
	IPC_OWNER        = 1 << 15 // NOLINT
	SYS_MODULE       = 1 << 16 // NOLINT
	SYS_RAWIO        = 1 << 17 // NOLINT
	SYS_CHROOT       = 1 << 18 // NOLINT
	SYS_PTRACE       = 1 << 19 // NOLINT
	SYS_PACCT        = 1 << 20 // NOLINT
	SYS_ADMIN        = 1 << 21 // NOLINT
	SYS_BOOT         = 1 << 22 // NOLINT
	SYS_NICE         = 1 << 23 // NOLINT
	SYS_RESOURCE     = 1 << 24 // NOLINT
	SYS_TIME         = 1 << 25 // NOLINT
	SYS_TTY_CONFIG   = 1 << 26 // NOLINT
	MKNOD            = 1 << 27 // NOLINT
	LEASE            = 1 << 28 // NOLINT
	AUDIT_WRITE      = 1 << 29 // NOLINT
	AUDIT_CONTROL    = 1 << 30 // NOLINT
	SETFCAP          = 1 << 31 // NOLINT
	MAC_OVERRIDE     = 1 << 32 // NOLINT
	MAC_ADMIN        = 1 << 33 // NOLINT
	SYSLOG           = 1 << 34 // NOLINT
	WAKE_ALARM       = 1 << 35 // NOLINT
	BLOCK_SUSPEND    = 1 << 36 // NOLINT
	AUDIT_READ       = 1 << 37 // NOLINT
)

// capMaskToString returns a string representation of a capability mask constant.
func capMaskToString(c uint64) string {
	switch c {
	case CHOWN:
		return "chown"
	case DAC_OVERRIDE:
		return "dac_override"
	case DAC_READ_SEARCH:
		return "read_search"
	case FOWNER:
		return "fowner"
	case FSETID:
		return "fsetid"
	case KILL:
		return "kill"
	case SETGID:
		return "setgid"
	case SETUID:
		return "setuid"
	case SETPCAP:
		return "setpcap"
	case LINUX_IMMUTABLE:
		return "linux_immutable"
	case NET_BIND_SERVICE:
		return "net_bind_service"
	case NET_BROADCAST:
		return "net_broadcast"
	case NET_ADMIN:
		return "net_admin"
	case NET_RAW:
		return "net_raw"
	case IPC_LOCK:
		return "ipc_lock"
	case IPC_OWNER:
		return "ipc_owner"
	case SYS_MODULE:
		return "sys_module"
	case SYS_RAWIO:
		return "sys_rawio"
	case SYS_CHROOT:
		return "sys_chroot"
	case SYS_PTRACE:
		return "sys_ptrace"
	case SYS_PACCT:
		return "sys_pacct"
	case SYS_ADMIN:
		return "sys_admin"
	case SYS_BOOT:
		return "sys_boot"
	case SYS_NICE:
		return "sys_nice"
	case SYS_RESOURCE:
		return "sys_resource"
	case SYS_TIME:
		return "sys_time"
	case SYS_TTY_CONFIG:
		return "sys_tty_config"
	case MKNOD:
		return "mknod"
	case LEASE:
		return "lease"
	case AUDIT_WRITE:
		return "audit_write"
	case AUDIT_CONTROL:
		return "audit_control"
	case SETFCAP:
		return "setfcap"
	case MAC_OVERRIDE:
		return "mac_override"
	case MAC_ADMIN:
		return "mac_admin"
	case SYSLOG:
		return "syslog"
	case WAKE_ALARM:
		return "wake_alarm"
	case BLOCK_SUSPEND:
		return "block_suspend"
	case AUDIT_READ:
		return "audit_read"
	}
	return fmt.Sprintf("unknown(%v)", int(c))
}

// Caps holds capability sets associated with an executable file.
type Caps struct {
	// See capabilities(7) for detailed definitions of how these fields are intepreted.
	Effective, Inheritable, Permitted uint64
}

// Empty returns true if no capabilities are present.
func (c Caps) Empty() bool {
	return c.Effective == 0 && c.Inheritable == 0 && c.Permitted == 0
}

// String returns a string representation of capabilities that are present, e.g. "[e:net_raw p:net_raw]".
func (c Caps) String() string {
	var parts []string
	add := func(prefix string, n uint64) {
		var names []string
		for i := 0; i < 64; i++ {
			mask := uint64(1) << uint32(i)
			if n&mask != 0 {
				names = append(names, capMaskToString(mask))
			}
		}
		if len(names) > 0 {
			parts = append(parts, prefix+strings.Join(names, "|"))
		}
	}

	add("e:", c.Effective)
	add("i:", c.Inheritable)
	add("p:", c.Permitted)
	return "[" + strings.Join(parts, " ") + "]"
}

// GetCaps returns Linux capabilities defined for the file at path.
// No error is returned if the filesystem does not support capabilities.
func GetCaps(path string) (Caps, error) {
	pathPtr, err := unix.BytePtrFromString(path)
	if err != nil {
		return Caps{}, err
	}
	// Defined in libcap's libcap/libcap.h.
	var capsStruct struct {
		magicEtc uint32
		data     [2]struct{ permitted, inheritable uint32 }
	}
	size, _, errno := unix.Syscall6(unix.SYS_LGETXATTR, uintptr(unsafe.Pointer(pathPtr)),
		uintptr(unsafe.Pointer(attrName)), uintptr(unsafe.Pointer(&capsStruct)), unsafe.Sizeof(capsStruct), 0, 0)

	// ENODATA is returned if there are no attributes, and ENOTSUP is returned for e.g. FUSE filesystems.
	if errno == unix.ENODATA || errno == unix.ENOTSUP {
		return Caps{}, nil
	}
	if errno != 0 {
		return Caps{}, errno
	}

	// Version 2, permitting 64-bit capability sets, was introduced in Linux 2.6.25.
	// Version 3, additionally encoding the root user ID of the namespace, was introduced in 4.14.
	// See capabilities(7) for more details.
	if version := capsStruct.magicEtc >> 24; version != 2 {
		return Caps{}, errors.Errorf("got version %v; want 2", version)
	}
	if size != unsafe.Sizeof(capsStruct) {
		return Caps{}, errors.Errorf("got %v byte(s); want %v", size, unsafe.Sizeof(capsStruct))
	}

	caps := Caps{
		Inheritable: uint64(capsStruct.data[1].inheritable)<<32 | uint64(capsStruct.data[0].inheritable),
		Permitted:   uint64(capsStruct.data[1].permitted)<<32 | uint64(capsStruct.data[0].permitted),
	}

	// Defined in linux/capability.h. From cap_get_file(3):
	// On Linux, the file Effective set is a single bit.  If it is enabled,
	// then all Permitted capabilities are enabled in the Effective set of the
	// calling process when the file is executed; otherwise, no capabilities
	// are enabled in the process's Effective set following an execve(2).
	const effectiveFlag = 0x000001
	if capsStruct.magicEtc&effectiveFlag != 0 {
		caps.Effective = caps.Permitted
	}

	return caps, nil
}
