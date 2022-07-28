// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package audioutils provides the util functions needed for the vm audio tests
package audioutils

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"os/user"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/testing"
)

const (
	cgroupPath string = "/sys/fs/cgroup/cpu/termina/tasks"
)

// Config includes all the params needed to setup crosvm for vm audio tests.
type Config struct {
	CrosvmArgs    []string
	VhostUserArgs []string
}

// RunCrosvm runs crosvm and the crosvm vhost user device if required.
func RunCrosvm(ctx context.Context, kernelPath, kernelLogPath string, kernelArgs []string, config Config) error {
	crosvmCmd, devCmd, cleanupFunc, err := crosvmCmd(ctx, kernelPath, kernelLogPath, kernelArgs, config)
	if err != nil {
		return errors.Wrap(err, "failed to get crosvm cmd")
	}
	defer cleanupFunc()

	if devCmd != nil {
		testing.ContextLog(ctx, "Starting crosvm device")
		if err = devCmd.Start(); err != nil {
			return errors.Wrap(err, "failed to start crosvm device")
		}
	}

	testing.ContextLog(ctx, "Launching crosvm")
	if err = crosvmCmd.Run(testexec.DumpLogOnError); err != nil {
		return errors.Wrap(err, "failed to run crosvm")
	}

	if devCmd != nil {
		if err = devCmd.Wait(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to complete vhost-user-snd-device")
		}
	}

	return nil
}

// crosvmCmd setups the crosvm and device command for audio device testing.
func crosvmCmd(ctx context.Context, kernelPath, kernelLogPath string, kernelArgs []string, config Config) (crosvmCmd, devCmd *testexec.Cmd, cleanupFunc func() error, retErr error) {
	cleanupFunc = func() error { return nil }
	defer func() {
		if retErr != nil {
			if err := cleanupFunc(); err != nil {
				testing.ContextLog(ctx, "Failed to cleanup: ", err)
			}
			cleanupFunc = nil
		}
	}()
	if len(config.VhostUserArgs) > 0 {
		tempDir, err := ioutil.TempDir("/usr/local/tmp", "CrosvmCmd.")
		if err != nil {
			return nil, nil, cleanupFunc, errors.Wrap(err, "failed to create temporary directory")
		}
		cleanupFunc = func() error {
			return os.RemoveAll(tempDir)
		}
		sock := filepath.Join(tempDir, "vhost-user-snd.sock")
		deviceArgs := append([]string{"device"}, config.VhostUserArgs...)
		deviceArgs = append(deviceArgs, "--socket", sock)
		devCmd = testexec.CommandContext(ctx, "crosvm", deviceArgs...)
		config.CrosvmArgs = append(config.CrosvmArgs, "--vhost-user-snd", sock)
	}

	kernelParams := []string{
		"root=/dev/root",
		"rootfstype=virtiofs",
		"rw",
	}
	kernelParams = append(kernelParams, kernelArgs...)

	crosvmArgs := []string{"crosvm", "run"}
	crosvmArgs = append(crosvmArgs, config.CrosvmArgs...)
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
	crosvmCmd = testexec.CommandContext(ctx, "sh", []string{"-c", strings.Join(cmdStr, " ")}...)

	if devCmd == nil {
		// Same effect as calling `newgrp cras` before `crosvm` in shell
		// This is needed to access /run/cras/.cras_socket (legacy socket)
		//
		// vhost-user device does not need this as it doesn't involve minijail.
		crasGrp, err := user.LookupGroup("cras")
		if err != nil {
			return nil, nil, cleanupFunc, errors.Wrap(err, "failed to find group id for cras")
		}
		crasGrpID, err := strconv.ParseUint(crasGrp.Gid, 10, 32)
		if err != nil {
			return nil, nil, cleanupFunc, errors.Wrap(err, "failed to convert cras grp id to integer")
		}
		crosvmCmd.Cred(syscall.Credential{
			Uid:         0,
			Gid:         0,
			Groups:      []uint32{uint32(crasGrpID)},
			NoSetGroups: false,
		})
	}

	return crosvmCmd, devCmd, cleanupFunc, nil
}
