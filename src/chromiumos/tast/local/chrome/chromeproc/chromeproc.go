// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromeproc provides utilities to find Chrome processes.
package chromeproc

import (
	"context"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// ExecPath contains the path to the Chrome executable.
const ExecPath = "/opt/google/chrome/chrome"

// crashpadExecPath contains the path to crashpad's binary. Though it is not
// the same executable as Chrome, it is spawned from Chrome and we consider as
// one of the Chrome processes.
const crashpadExecPath = "/opt/google/chrome/crashpad_handler"

// Version returns the Chrome browser version. E.g. Chrome version W.X.Y.Z will be reported as a list of strings.
func Version(ctx context.Context) ([]string, error) {
	versionStr, err := testexec.CommandContext(ctx, ExecPath, "--version").Output(testexec.DumpLogOnError)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get chrome version")
	}

	versionRE := regexp.MustCompile(`(\d+)\.(\d+)\.(\d+)\.(\d+)`)
	matches := versionRE.FindStringSubmatch(string(versionStr))
	if len(matches) <= 1 {
		return nil, errors.Errorf("can't recognize version string: %s", string(versionStr))
	}
	return matches[1:], nil
}

// GetPIDs returns all PIDs corresponding to Chrome processes (including
// crashpad's handler).
func GetPIDs() ([]int, error) {
	all, err := process.Pids()
	if err != nil {
		return nil, err
	}

	pids := make([]int, 0)
	for _, pid := range all {
		if proc, err := process.NewProcess(pid); err != nil {
			// Assume that the process exited.
			continue
		} else if exe, err := proc.Exe(); err == nil && (exe == ExecPath || exe == crashpadExecPath) {
			pids = append(pids, int(pid))
		}
	}
	return pids, nil
}

// GetRootPID returns the PID of the root Chrome process.
// This corresponds to the browser process.
func GetRootPID() (int, error) {
	pids, err := GetPIDs()
	if err != nil {
		return -1, err
	}

	for _, pid := range pids {
		// If we see errors, assume that the process exited.
		proc, err := process.NewProcess(int32(pid))
		if err != nil {
			continue
		}

		// crashpad is never the root browser process.
		if exe, err := proc.Exe(); err != nil || exe == crashpadExecPath {
			continue
		}

		// A browser process should not have --type= flag.
		// This check alone is not enough to determine that proc is a browser process;
		// it might be a brand-new process that just forked from the browser process.
		if cmdline, err := proc.Cmdline(); err != nil || strings.Contains(cmdline, " --type=") {
			continue
		}

		// A browser process should have session_manager as its parent process.
		// This check alone is not enough to determine that proc is a browser process;
		// due to the use of prctl(PR_SET_CHILD_SUBREAPER) in session_manager,
		// when the browser process exits, non-browser processes can temporarily
		// become children of session_manager.
		ppid, err := proc.Ppid()
		if err != nil || ppid <= 0 {
			continue
		}
		pproc, err := process.NewProcess(ppid)
		if err != nil {
			continue
		}
		if exe, err := pproc.Exe(); err != nil || exe != "/sbin/session_manager" {
			continue
		}

		// It is still possible that proc is not a browser process if the browser
		// process exited immediately after it forked, but it is fairly unlikely.
		return pid, nil
	}
	return -1, errors.New("root not found")
}

// getProcesses returns Chrome processes with the --type=${t} flag.
func getProcesses(t string) ([]*process.Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, err
	}

	// Wrap by whitespaces. Please see the comment below.
	flg := " --type=" + t + " "
	// Or accept the --type= flag on the end of the command line.
	endFlg := " --type=" + t
	var ret []*process.Process
	for _, proc := range ps {
		if exe, err := proc.Exe(); err != nil || exe != ExecPath {
			continue
		}

		// Process.CmdlineSliceWithContext() is more appropriate, but
		// 1) Chrome's /proc/*/cmdline is whitespace separated, so
		//    proc.CmdlineSlice/CmdlineSliceWithContext won't work.
		//    cf) https://bugs.gentoo.org/477538
		// 2) Our gopsutil is too old so that CmdlineSliceWithContext
		//    is not supported.
		// Thus, instead Cmdline() is used here. Please also find
		// whitespaces in |flg|.
		// cf) crbug.com/887875
		cmd, err := proc.Cmdline()
		if err != nil {
			continue
		}
		if strings.Contains(cmd, flg) || strings.HasSuffix(cmd, endFlg) {
			ret = append(ret, proc)
		}
	}
	return ret, nil
}

// GetPluginProcesses returns Chrome plugin processes.
func GetPluginProcesses() ([]*process.Process, error) {
	return getProcesses("plugin")
}

// GetRendererProcesses returns Chrome renderer processes.
func GetRendererProcesses() ([]*process.Process, error) {
	return getProcesses("renderer")
}

// GetGPUProcesses returns Chrome gpu-process processes.
func GetGPUProcesses() ([]*process.Process, error) {
	return getProcesses("gpu-process")
}

// GetBrokerProcesses returns Chrome broker processes.
func GetBrokerProcesses() ([]*process.Process, error) {
	return getProcesses("broker")
}
