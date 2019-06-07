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
	procs, err := process.Processes()
	if err != nil {
		return -1, errors.Wrap(err, "failed to list processes")
	}

	initPath := "/usr/bin/crosvm"
	procFilter := func(proc *process.Process) bool { return true }

	if guest == container {
		uid, err := sysutil.GetUID("android-root")
		if err != nil {
			return -1, err
		}

		initPath = "/init"
		procFilter = func(proc *process.Process) bool {
			uids, err := proc.Uids()
			return err == nil && uint32(uids[0]) == uid
		}
	}

	for _, p := range procs {
		if !procFilter(p) {
			continue
		}
		if exe, err := p.Exe(); err == nil && exe == initPath {
			return p.Pid, nil
		}
	}

	return -1, errors.New("didn't find init process")
}
