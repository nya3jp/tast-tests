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

// getUserPath returns the user and the path to the entry point of ARC
func getUserPath() (user, path string, err error) {
	vm, err := VMEnabled()
	if err != nil {
		return "", "", errors.Wrap(err, "failed to determine if ARCVM is enabled")
	}
	if vm {
		return "crosvm", "/usr/bin/crosvm", nil
	}

	return "android-root", "/init", nil
}

// InitPID returns the PID (outside the guest) of the ARC init process.
// It returns an error in case process is not found.
func InitPID() (int32, error) {
	u, initPath, err := getUserPath()
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
			if exe, err := p.Exe(); err == nil && exe == initPath {
				return p.Pid, nil
			}
		}
	}

	return -1, errInitNotFound
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
