// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/sysutil"
)

// InitPID returns the PID (outside the container) of the ARC init process.
func InitPID() (int32, error) {
	uid, err := sysutil.GetUID("android-root")
	if err != nil {
		return -1, err
	}

	procs, err := process.Processes()
	if err != nil {
		return -1, errors.Wrap(err, "failed to list processes")
	}

	const initPath = "/init"
	for _, p := range procs {
		if uids, err := p.Uids(); err == nil && uint32(uids[0]) == uid {
			if exe, err := p.Exe(); err == nil && exe == initPath {
				return p.Pid, nil
			}
		}
	}
	return -1, errors.New("didn't find init process")
}
