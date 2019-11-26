// Copyright 2017 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package chrome

import (
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/errors"
)

// ExecPath contains the path to the Chrome executable.
const ExecPath = "/opt/google/chrome/chrome"

// The cache of the information related to GetPIDs() / GetRootPID().
type pidCache struct {
	// the mapping from PID to whether it is a chrome process or not.
	pids    map[int32]bool
	rootPID int32
}

func isProcessChrome(proc *process.Process) bool {
	exe, err := proc.Exe()
	// When error happens, the process might have exited already, or Tast does not
	// have the access to the information. Either way, it is not a Chrome process.
	return err == nil && exe == ExecPath
}

func isProcessChromeRoot(proc *process.Process) bool {
	// A browser process should not have --type= flag.
	// This check alone is not enough to determine that proc is a browser process;
	// it might be a brand-new process that just forked from the browser process.
	if cmdline, err := proc.Cmdline(); err != nil || strings.Contains(cmdline, " --type=") {
		return false
	}

	// A browser process should have session_manager as its parent process.
	// This check alone is not enough to determine that proc is a browser process;
	// due to the use of prctl(PR_SET_CHILD_SUBREAPER) in session_manager,
	// when the browser process exits, non-browser processes can temporarily
	// become children of session_manager.
	ppid, err := proc.Ppid()
	if err != nil || ppid <= 0 {
		return false
	}
	pproc, err := process.NewProcess(ppid)
	if err != nil {
		return false
	}

	exe, err := pproc.Exe()

	// It is still possible that proc is not a browser process if the browser
	// process exited immediately after it forked, but it is fairly unlikely.
	return err == nil && exe == "/sbin/session_manager"
}

// getPIDs returns the PIDs of Chrome processes. This method updates the
// cache, and the next invocation will reuse the existing cache. This means
// technically there's a chance that a new process is created and assigned to
// the same PID as an old one between subsequent runs of getPIDs(). The caller
// should make sure to invoke this method frequently enough to reduce the risk
// of using the wrong cache.
func (c *pidCache) getPIDs() ([]int, error) {
	all, err := process.Pids()
	if err != nil {
		return nil, err
	}
	oldRootPID := c.rootPID
	c.rootPID = -1
	pids := make([]int, 0)
	newCache := make(map[int32]bool, len(all))
	for _, pid := range all {
		if isChrome, ok := c.pids[pid]; ok {
			if isChrome {
				pids = append(pids, int(pid))
				if pid == oldRootPID {
					if c.rootPID != -1 {
						return nil, errors.New("multiple root processes found")
					}
					c.rootPID = pid
				}
			}
			newCache[pid] = isChrome
			continue
		}

		isChrome := false

		if proc, err := process.NewProcess(pid); err == nil {
			isChrome = isProcessChrome(proc)
			if isChrome {
				if isProcessChromeRoot(proc) {
					if c.rootPID != -1 {
						return nil, errors.New("multiple root processes found")
					}
					c.rootPID = pid
				}
			}
		}
		if isChrome {
			pids = append(pids, int(pid))
		}
		newCache[pid] = isChrome
	}
	c.pids = newCache
	return pids, nil
}

func (c *pidCache) getRootPID() (int, error) {
	_, err := c.getPIDs()
	if err != nil {
		return -1, err
	}
	if c.rootPID == -1 {
		return -1, errors.New("root process not found")
	}
	return int(c.rootPID), nil
}

// GetPIDs returns all PIDs corresponding to Chrome processes.
func GetPIDs() ([]int, error) {
	c := &pidCache{}
	return c.getPIDs()
}

// GetRootPID returns the PID of the root Chrome process.
// This corresponds to the browser process.
func GetRootPID() (int, error) {
	c := &pidCache{}
	return c.getRootPID()
}

// getProcesses returns Chrome processes with the --type=${t} flag.
func getProcesses(t string) ([]process.Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, err
	}

	// Wrap by whitespaces. Please see the comment below.
	flg := " --type=" + t + " "
	// Or accept the --type= flag on the end of the command line.
	endFlg := " --type=" + t
	var ret []process.Process
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
			ret = append(ret, *proc)
		}
	}
	return ret, nil
}

// GetPluginProcesses returns Chrome plugin processes.
func GetPluginProcesses() ([]process.Process, error) {
	return getProcesses("plugin")
}

// GetRendererProcesses returns Chrome renderer processes.
func GetRendererProcesses() ([]process.Process, error) {
	return getProcesses("renderer")
}

// GetGPUProcesses returns Chrome gpu-process processes.
func GetGPUProcesses() ([]process.Process, error) {
	return getProcesses("gpu-process")
}

// GetBrokerProcesses returns Chrome broker processes.
func GetBrokerProcesses() ([]process.Process, error) {
	return getProcesses("broker")
}
