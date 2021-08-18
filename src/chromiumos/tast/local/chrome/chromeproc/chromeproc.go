// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromeproc provides utilities to find Chrome processes.
package chromeproc

import (
	"context"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/process"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
)

// installDir is the path to the directory that contains Chrome executable.
const installDir = "/opt/google/chrome"

// ExecPath contains the path to the Chrome executable.
const ExecPath = "/opt/google/chrome/chrome"

const chromeExe = "chrome"

// crashpadHandlerExe is the name of executable. Though it is not
// the same executable as Chrome, it is spawned from Chrome and we consider as
// one of the Chrome processes.
const crashpadHandlerExe = "chrome_crashpad_handler"

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

// processes returns an array of Processes that satisfies the given filter.
func processes(filter func(p *process.Process) bool) ([]*process.Process, error) {
	ps, err := process.Processes()
	if err != nil {
		return nil, errors.Wrap(err, "failed to obtain processes")
	}

	var ret []*process.Process
	for _, p := range ps {
		if filter(p) {
			ret = append(ret, p)
		}
	}
	return ret, nil
}

// Processes returns all Chrome related processes, which includes "chrome" processes
// and "chrome_crashpad_handler" processes.
// dir is the path to the directory containing those executables.
// TODO(crbug.com/1237972): This is being moved to internal package, so nobody
// outside of the package should depend on this.
func Processes(dir string) ([]*process.Process, error) {
	absdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert %q to abs path", dir)
	}

	crPath := filepath.Join(absdir, chromeExe)
	cphPath := filepath.Join(absdir, crashpadHandlerExe)
	return processes(func(p *process.Process) bool {
		exe, err := p.Exe()
		if err != nil {
			return false
		}
		return exe == crPath || exe == cphPath
	})
}

// GetPIDs returns all PIDs corresponding to Chrome processes (including
// crashpad's handler).
func GetPIDs() ([]int, error) {
	ps, err := Processes(installDir)
	if err != nil {
		return nil, err
	}

	var pids []int
	for _, p := range ps {
		pids = append(pids, int(p.Pid))
	}
	return pids, nil
}

// GetRootPID returns the PID of the root Chrome process.
// This corresponds to the browser process.
func GetRootPID() (int, error) {
	p, err := Root(ExecPath)
	if err != nil {
		return -1, err
	}

	return int(p.Pid), nil
}

// Root returns Process instance for Chrome's root process (i.e. Browser process).
// TODO(crbug.com/1237972): This is being moved to internal package, so nobody
// outside of the package should depend on this.
func Root(dir string) (*process.Process, error) {
	absdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert %q to abs path", dir)
	}

	path := filepath.Join(absdir, "chrome")
	ps, err := processes(func(p *process.Process) bool {
		// The exec path should match.
		if exe, err := p.Exe(); err != nil || exe != path {
			return false
		}

		// A browser process should not have --type= flag.
		// This check alone is not enough to determine that proc is a browser process;
		// it might be a brand-new process that just forked from the browser process.
		if cmdline, err := p.Cmdline(); err != nil || strings.Contains(cmdline, " --type=") {
			return false
		}

		// A browser process should have session_manager as its parent process.
		// This check alone is not enough to determine that proc is a browser process;
		// due to the use of prctl(PR_SET_CHILD_SUBREAPER) in session_manager,
		// when the browser process exits, non-browser processes can temporarily
		// become children of session_manager.
		ppid, err := p.Ppid()
		if err != nil || ppid <= 0 {
			return false
		}

		pproc, err := process.NewProcess(ppid)
		if err != nil {
			return false
		}
		if exe, err := pproc.Exe(); err != nil || exe != "/sbin/session_manager" {
			return false
		}

		// It is still possible that proc is not a browser process if the browser
		// process exited immediately after it forked, but it is fairly unlikely.
		return true
	})
	if err != nil {
		return nil, err
	}
	if len(ps) == 0 {
		return nil, errors.New("root not found")
	}
	if len(ps) != 1 {
		// This is the case explained at the end of the filter function.
		return nil, errors.Errorf("unexpected number of chrome root processes: got %d, want 1", len(ps))
	}

	return ps[0], nil
}

// processesByArgs returns Chrome processes whose command line args match the given re.
func processesByArgs(dir string, re *regexp.Regexp) ([]*process.Process, error) {
	absdir, err := filepath.Abs(dir)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert %q to abs path", dir)
	}

	path := filepath.Join(absdir, chromeExe)
	return processes(func(p *process.Process) bool {
		if exe, err := p.Exe(); err != nil || exe != path {
			return false
		}

		// Process.CmdlineSliceWithContext() is more appropriate, but
		// 1) Chrome's /proc/*/cmdline is whitespace separated, so
		//    p.CmdlineSlice/CmdlineSliceWithContext won't work.
		//    cf) https://bugs.gentoo.org/477538
		// 2) Our gopsutil is too old so that CmdlineSliceWithContext
		//    is not supported.
		// Thus, instead Cmdline() is used here. Please also find
		// whitespaces in |flg|.
		// cf) crbug.com/887875
		cmd, err := p.Cmdline()
		if err != nil {
			return false
		}
		return re.MatchString(cmd)
	})
}

var (
	pluginRE   = regexp.MustCompile(` --type=plugin(?: |$)`)
	rendererRE = regexp.MustCompile(` --type=renderer(?: |$)`)
	gpuRE      = regexp.MustCompile(` --type=gpu-process(?: |$)`)
	brokerRE   = regexp.MustCompile(` --type=broker(?: |$)`)
)

// PluginProcesses returns Chrome plugin processes.
// TODO(crbug.com/1237972): This is being moved to internal package, so nobody
// outside of the package should depend on this.
func PluginProcesses(dir string) ([]*process.Process, error) {
	return processesByArgs(dir, pluginRE)
}

// RendererProcesses returns Chrome renderer processes.
// TODO(crbug.com/1237972): This is being moved to internal package, so nobody
// outside of the package should depend on this.
func RendererProcesses(dir string) ([]*process.Process, error) {
	return processesByArgs(dir, rendererRE)
}

// GPUProcesses returns Chrome gpu-process processes.
// TODO(crbug.com/1237972): This is being moved to internal package, so nobody
// outside of the package should depend on this.
func GPUProcesses(dir string) ([]*process.Process, error) {
	return processesByArgs(dir, gpuRE)
}

// BrokerProcesses returns Chrome broker processes.
// TODO(crbug.com/1237972): This is being moved to internal package, so nobody
// outside of the package should depend on this.
func BrokerProcesses(dir string) ([]*process.Process, error) {
	return processesByArgs(dir, brokerRE)
}

// GetPluginProcesses returns Chrome plugin processes.
func GetPluginProcesses() ([]*process.Process, error) {
	return PluginProcesses(installDir)
}

// GetRendererProcesses returns Chrome renderer processes.
func GetRendererProcesses() ([]*process.Process, error) {
	return RendererProcesses(installDir)
}

// GetGPUProcesses returns Chrome gpu-process processes.
func GetGPUProcesses() ([]*process.Process, error) {
	return GPUProcesses(installDir)
}

// GetBrokerProcesses returns Chrome broker processes.
func GetBrokerProcesses() ([]*process.Process, error) {
	return BrokerProcesses(installDir)
}
