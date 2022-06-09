// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package chromeproc provides utilities to find Chrome processes.
package chromeproc

import (
	"context"
	"path/filepath"
	"regexp"

	"github.com/shirou/gopsutil/v3/process"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/procutil"
	"chromiumos/tast/testing"
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

var (
	nonRootRE = regexp.MustCompile(
		// Browser process must not have --type= arguments.
		// Also, if it's running with --help, --h, --product-version, --version, --credits
		// the process outputs some info, and terminates soon. It is not the browser process.
		// One of the actual use case is that crash_reporter executes chrome with
		// --product-version to obtain its version info.
		` --?(type=\S+|help|h|product-version|version|credits)( |$)`)
	pluginRE   = regexp.MustCompile(` --?type=plugin(?: |$)`)
	rendererRE = regexp.MustCompile(` --?type=renderer(?: |$)`)
	gpuRE      = regexp.MustCompile(` --?type=gpu-process(?: |$)`)
	brokerRE   = regexp.MustCompile(` --?type=broker(?: |$)`)
)

// Root returns Process instance for Chrome's root process (i.e. Browser process).
func Root(execPath string) (*process.Process, error) {
	return RootWithContext(nil, execPath)
}

// RootWithContext is almost same as Root, but takes context.Context for logging purpose.
func RootWithContext(ctx context.Context, execPath string) (*process.Process, error) {
	if !filepath.IsAbs(execPath) {
		return nil, errors.Errorf("execPath %q must be abs path", execPath)
	}
	return procutil.FindUnique(procutil.And(procutil.ByExe(execPath), func(p *process.Process) bool {
		// Check if the process is running in the browser process mode.
		// This check alone is not enough to determine that proc is a browser process;
		// it might be a brand-new process that just forked from the browser process.
		cmdline, err := p.Cmdline()
		if err != nil || nonRootRE.MatchString(cmdline) {
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
		pExe, err := pproc.Exe()
		if err != nil || pExe == execPath {
			return false
		}

		// It is still possible that proc is not a browser process if the browser
		// process exited immediately after it forked, but it is fairly unlikely.

		// Currently, we're facing mysterious error that there are
		// multiple processes matching this condition, but we're not yet sure
		// what are they. Dumping the log of what we checked here to investigate
		// further.
		if ctx != nil {
			// Note that it is important to use local variable, instead of calling p.*
			// again, because the process maybe terminated during this short period.
			testing.ContextLogf(ctx, "Found browser process: pid=%d, cmdline=%q, ppid=%d, pExe=%q", p.Pid, cmdline, ppid, pExe)
		}
		return true
	}))
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
