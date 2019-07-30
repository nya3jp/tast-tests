// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
)

// InitPID returns the PID (outside the guest) of the ARC init process.
func InitPID() (int32, error) {
	u := "android-root"
	initPath := "/init"

	if vm, err := VMEnabled(); err != nil {
		return -1, errors.Wrap(err, "failed to determine if ARCVM is enabled")
	} else if vm {
		u = "crosvm"
		initPath = "/usr/bin/crosvm"
	}

	if ver, err := SDKVersion(); err != nil {
		return -1, errors.Wrap(err, "failed to get SDK version")
	} else if ver >= SDKQ {
		initPath = "/system/bin/init"
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
	return -1, errors.New("didn't find init process")
}
