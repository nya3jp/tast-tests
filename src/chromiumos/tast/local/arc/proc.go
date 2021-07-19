// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
)

var errInitNotFound = errors.New("didn't find init process")

// Daemon references one of the system daemons
type Daemon int

const (
	// Init references the init process daemon
	Init = iota
	// MojoProxy references the arcvm_server_proxy daemon
	MojoProxy
)

// getUserPath returns the user and the path to the specified daemon
func getUserPath(daemon Daemon) (user, path string, err error) {
	switch daemon {
	case Init:
		vm, err := VMEnabled()
		if err != nil {
			return "", "", errors.Wrap(err, "failed to determine if ARCVM is enabled")
		}
		if vm {
			return "crosvm", "/usr/bin/crosvm", nil
		}
		return "android-root", "/init", nil
	case MojoProxy:
		return "root", "/usr/bin/arcvm_server_proxy", nil
	default:
		return "", "", errors.Wrapf(err, "failed to indentify daemon %d", daemon)
	}
}

// GetDaemonPID returns the PID (outside the guest) of the
// specified daemon process.
func GetDaemonPID(daemon Daemon) (int32, error) {
	u, path, err := getUserPath(daemon)
	if err != nil {
		return -1, err
	}

	uid, err := sysutil.GetUID(u)
	if err != nil {
		return -1, err
	}

	procs, err := process.Processes()
	if err != nil {
		return -1, errors.Wrap(err, "failed to list processes")
	}

	for _, p := range procs {
		if uids, err := p.Uids(); err == nil && uint32(uids[0]) == uid {
			if exe, err := p.Exe(); err == nil && exe == path {
				return p.Pid, nil
			}
		}
	}

	return -1, errInitNotFound
}

// InitPID returns the PID (outside the guest) of the ARC init process.
// It returns an error in case process is not found.
func InitPID() (int32, error) {
	return GetDaemonPID(init)
}

// InitExists returns true in case ARC init process exists.
func InitExists() (bool, error) {
	_, err := InitPID()
	if err != nil {
		if errors.Is(err, errInitNotFound) {
			return false, nil
		}
		return false, err
	}
	return true, nil
}
