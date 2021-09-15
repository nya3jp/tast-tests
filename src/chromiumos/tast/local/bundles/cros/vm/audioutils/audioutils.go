// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audioutils provides the util functions needed for the vm audio tests
package audioutils

import (
	"context"
	"fmt"
	"os/user"
	"strconv"
	"strings"
	"syscall"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

const (
	cgroupPath string = "/sys/fs/cgroup/cpu/vms/termina/tasks"
)

// CrosvmCmd setups the crosvm command for audio device testing
func CrosvmCmd(ctx context.Context, kernelPath, kernelLogPath string, kernelArgs, deviceArgs []string) (*testexec.Cmd, error) {
	kernelParams := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
	}
	kernelParams = append(kernelParams, kernelArgs...)

	crosvmArgs := []string{"crosvm", "run"}
	crosvmArgs = append(crosvmArgs, deviceArgs...)
	crosvmArgs = append(crosvmArgs,
		"-p", "\""+strings.Join(kernelParams, " ")+"\"",
		"--serial", fmt.Sprintf("type=file,num=1,console=true,path=%s", kernelLogPath),
		"--shared-dir", "/:/dev/root:type=fs:cache=always",
		kernelPath)

	// Add the shell process id to the control group
	cmdStr := []string{"echo $$ >", cgroupPath, "&&"}
	// Set the rtprio limit of the shell process to unlimited.
	cmdStr = append(cmdStr, "prlimit", "--pid", "$$", "--rtprio=unlimited", "&&")
	cmdStr = append(cmdStr, crosvmArgs...)
	cmd := testexec.CommandContext(ctx, "sh", []string{"-c", strings.Join(cmdStr, " ")}...)

	// Same effect as calling `newgrp cras` before `crosvm` in shell
	// This is needed to access /run/cras/.cras_socket (legacy socket)
	crasGrp, err := user.LookupGroup("cras")
	if err != nil {
		return nil, errors.Wrap(err, "failed to find group id for cras")
	}
	crasGrpID, err := strconv.ParseUint(crasGrp.Gid, 10, 32)
	if err != nil {
		return nil, errors.Wrap(err, "failed to convert cras grp id to integer")
	}
	cmd.SysProcAttr = &syscall.SysProcAttr{
		Credential: &syscall.Credential{
			Uid:         0,
			Gid:         0,
			Groups:      []uint32{uint32(crasGrpID)},
			NoSetGroups: false,
		},
	}

	return cmd, nil
}
