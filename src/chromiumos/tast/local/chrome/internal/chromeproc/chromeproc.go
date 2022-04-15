// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromeproc provides utilities to find Chrome processes.
package chromeproc

import (
	"path/filepath"
	"regexp"
	"strings"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/procutil"
)

const (
	chromeExe = "chrome"

	// crashpadHandlerExe is the name of executable. Though it is not
	// the same executable as Chrome, it is spawned from Chrome and we consider as
	// one of the Chrome processes.
	crashpadHandlerExe = "chrome_crashpad_handler"
)

// processes returns an array of Chrome Processes at execPath that satisfies the given filter.
func processes(execPath string, filter func(p *process.Process) bool) ([]*process.Process, error) {
	if !filepath.IsAbs(execPath) {
		return nil, errors.Errorf("execPath %q must be abs path", execPath)
	}
	return procutil.FindAll(procutil.And(procutil.ByExe(execPath), filter))
}

// Processes returns all Chrome processes.
// execPath is the abspath to the chrome executable.
func Processes(execPath string) ([]*process.Process, error) {
	return processes(execPath, func(p *process.Process) bool {
		return true
	})
}

// Root returns Process instance for Chrome's root process (i.e. Browser process).
func Root(execPath string) (*process.Process, error) {
	ps, err := processes(execPath, func(p *process.Process) bool {
		// A browser process should not have --type= flag.
		// This check alone is not enough to determine that proc is a browser process;
		// it might be a brand-new process that just forked from the browser process.
		if cmdline, err := p.Cmdline(); err != nil || strings.Contains(cmdline, " --type=") {
			return false
		}

		// A browser process should be spawned from some other executable process.
		// If it is ash-chrome, we expect it is forked from /sbin/session_manager.
		// If it is lacros-chrome, we expect it is forked from ash-chrome on production
		// or tast test executable for testing.
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
		if exe, err := pproc.Exe(); err != nil || exe == execPath {
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
func processesByArgs(execPath string, re *regexp.Regexp) ([]*process.Process, error) {
	return processes(execPath, func(p *process.Process) bool {
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
func PluginProcesses(execPath string) ([]*process.Process, error) {
	return processesByArgs(execPath, pluginRE)
}

// RendererProcesses returns Chrome renderer processes.
func RendererProcesses(execPath string) ([]*process.Process, error) {
	return processesByArgs(execPath, rendererRE)
}

// GPUProcesses returns Chrome gpu-process processes.
func GPUProcesses(execPath string) ([]*process.Process, error) {
	return processesByArgs(execPath, gpuRE)
}

// BrokerProcesses returns Chrome broker processes.
func BrokerProcesses(execPath string) ([]*process.Process, error) {
	return processesByArgs(execPath, brokerRE)
}
