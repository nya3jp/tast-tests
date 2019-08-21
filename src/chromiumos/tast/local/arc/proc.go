// Copyright 2019 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"strings"

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

// GetNewestPID returns the newest PID with name.
func GetNewestPID(name string) (int, error) {
	procs, err := process.Processes()
	if err != nil {
		return 0, err
	}
	var mostRecentMatch *process.Process
	var mostRecentCreateTime int64
	for _, proc := range procs {
		if cl, err := proc.Cmdline(); err != nil || !strings.Contains(cl, name) {
			continue
		}
		createTime, err := proc.CreateTime()
		if err != nil {
			continue
		}
		if mostRecentMatch == nil || createTime > mostRecentCreateTime {
			mostRecentMatch = proc
			mostRecentCreateTime = createTime
		}
	}
	if mostRecentMatch == nil {
		return 0, errors.Errorf("unable to find process with name %v", name)
	}
	return int(mostRecentMatch.Pid), nil
}
