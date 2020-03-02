// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher implements a library used to setup and launch linux-chrome.
package launcher

import (
	"context"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// BinaryPath is the root directory for linux-chrome related binaries.
const BinaryPath = LacrosTestPath + "/lacros_binary"

// linuxChrome contains all state associated with a linux-chrome instance
// that has been launched. Must call Close() to release resources.
type linuxChrome struct {
	Devsess *cdputil.Session // Debugging session for linux-chrome
	cmd     *testexec.Cmd    // The command context used to start linux-chrome.
}

// Close kills a launched instance of linux-chrome.
func (l *linuxChrome) Close(ctx context.Context) error {
	if l.Devsess != nil {
		l.Devsess.Close(ctx)
		l.Devsess = nil
	}
	if l.cmd != nil {
		if err := l.cmd.Cmd.Process.Kill(); err != nil {
			testing.ContextLog(ctx, "Failed to kill linux-chrome: ", err)
		}
		l.cmd.Cmd.Wait()
		l.cmd = nil
	}
	killLinuxChrome(ctx)
	return nil
}

// PidsFromPath returns the pids of all processes with a given path in their
// command line. This is typically used to find all chrome-related binaries,
// e.g. chrome, nacl_helper, etc. They typically share a path, even though their
// binary names differ.
// There may be a race condition between calling this method and using the pids
// later. It's possible that one of the processes is killed, and possibly even
// replaced with a process with the same pid.
func PidsFromPath(ctx context.Context, path string) ([]int, error) {
	all, err := process.Pids()
	if err != nil {
		return nil, err
	}

	pids := make([]int, 0)
	for _, pid := range all {
		if proc, err := process.NewProcess(pid); err != nil {
			// Assume that the process exited.
			continue
		} else if exe, err := proc.Exe(); err == nil && strings.Contains(exe, path) {
			pids = append(pids, int(pid))
		}
	}
	return pids, nil
}

// killLinuxChrome kills all binaries whose executable contains the base path
// to linux-chrome.
func killLinuxChrome(ctx context.Context) {
	// Kills all instances of linux-chrome and other related executables.
	pids, err := PidsFromPath(ctx, BinaryPath)
	if err != nil {
		testing.ContextLog(ctx, "Error finding pids for linux-chrome: ", err)
	}
	for _, pid := range pids {
		// We ignore errors, since it's possible the process has
		// already been killed.
		unix.Kill(pid, syscall.SIGKILL)
	}
}

// LaunchLinuxChrome launches a fresh instance of linux-chrome.
func LaunchLinuxChrome(ctx context.Context, p PreData) (*linuxChrome, error) {
	killLinuxChrome(ctx)

	// Create a new temporary directory for user data dir. We don't bother
	// clearing it on shutdown, since it's a subdirectory of the binary
	// path, which is cleared by pre.go. We need to use a new temporary
	// directory for each invocation so that successive calls to
	// LaunchLinuxChrome don't interfere with each other.
	userDataDir, err := ioutil.TempDir(BinaryPath, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}

	args := []string{
		"--ozone-platform=wayland",                                  // Use wayland to connect to exo wayland server.
		"--no-sandbox",                                              // Disable sandbox for now
		"--remote-debugging-port=0",                                 // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",                      // Allow Chrome to use the Chrome Automation API.
		"--whitelisted-extension-id=" + p.Chrome.TestExtID(),        // Whitelists the test extension to access all Chrome APIs.
		"--load-extension=" + strings.Join(p.Chrome.ExtDirs(), ","), // Load extensions
		"--no-first-run",                                            // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--user-data-dir=" + userDataDir,                            // Specify a --user-data-dir, which holds on-disk state for Chrome.
		"--lang=en-US",                                              // Language
		"--breakpad-dump-location=" + BinaryPath,                    // Specify location for breakpad dump files.
		"--window-size=800,600",
		"--log-file=" + userDataDir + "/logfile", // Specify log file location for debugging.
		"--enable-logging",                       // This flag is necessary to ensure the log file is written.
		"--enable-gpu-rasterization",             // Enable GPU rasterization. This is necessary to enable OOP rasterization.
		"--enable-oop-rasterization",             // Enable OOP rasterization.
		"about:blank",                            // Specify first tab to load.
	}

	l := &linuxChrome{}
	l.cmd = testexec.CommandContext(ctx, BinaryPath+"/chrome", args...)
	l.cmd.Cmd.Env = append(os.Environ(), "EGL_PLATFORM=surfaceless", "XDG_RUNTIME_DIR=/run/chrome")
	testing.ContextLog(ctx, "Starting chrome: ", strings.Join(args, " "))
	if err := l.cmd.Cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to launch linux-chrome")
	}

	debuggingPortPath := userDataDir + "/DevToolsActivePort"
	if l.Devsess, err = cdputil.NewSession(ctx, debuggingPortPath); err != nil {
		l.Close(ctx)
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}

	return l, nil
}
