// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher implements a library used to setup and launch linux-chrome.
package launcher

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"

	"github.com/mafredri/cdp/protocol/target"
	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// BinaryPath is the root directory for linux-chrome related binaries.
const BinaryPath = LacrosTestPath + "/lacros_binary"

// linuxChrome contains all state associated with a linux-chrome instance
// that has been launched. Must call Close() to release resources.
type linuxChrome struct {
	Devsess     *cdputil.Session // Debugging session for linux-chrome
	cmd         *testexec.Cmd    // The command context used to start linux-chrome.
	logMaster   *jslog.Master    // collects JS console output
	testExtID   string           // ID for test extension exposing APIs
	testExtConn *chrome.Conn     // connection to test extension exposing APIs
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

	l := &linuxChrome{testExtID: p.Chrome.TestExtID()}
	args := []string{
		"--ozone-platform=wayland",                                  // Use wayland to connect to exo wayland server.
		"--no-sandbox",                                              // Disable sandbox for now
		"--remote-debugging-port=0",                                 // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",                      // Allow Chrome to use the Chrome Automation API.
		"--whitelisted-extension-id=" + l.testExtID,                 // Whitelists the test extension to access all Chrome APIs.
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
		chrome.BlankURL,                          // Specify first tab to load.
	}

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

	l.logMaster = jslog.NewMaster()

	return l, nil
}

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm.
func (l *linuxChrome) NewConnForTarget(ctx context.Context, tm chrome.TargetMatcher) (*chrome.Conn, error) {
	t, err := chrome.FindTarget(ctx, l.Devsess, tm)
	if err != nil {
		return nil, err
	}

	return l.newConnInternal(ctx, t.TargetID, t.URL)
}

func (l *linuxChrome) newConnInternal(ctx context.Context, id target.ID, url string) (*chrome.Conn, error) {
	conn, err := chrome.NewConn(ctx, l.Devsess, id, l.logMaster, url, func(err error) error { return err })
	if err != nil {
		return nil, err
	}

	if url != "" && url != chrome.BlankURL {
		if err := conn.WaitForExpr(ctx, fmt.Sprintf("location.href !== %q", chrome.BlankURL)); err != nil {
			return nil, errors.Wrap(err, "failed to wait for navigation")
		}
	}
	if err := conn.WaitForExpr(ctx, "document.readyState === 'complete'"); err != nil {
		return nil, errors.Wrap(err, "failed to wait for loading")
	}
	return conn, nil
}

func (l *linuxChrome) TestAPIConn(ctx context.Context) (*chrome.TestConn, error) {
	if l.testExtConn != nil {
		return &chrome.TestConn{Conn: l.testExtConn}, nil
	}

	bgURL := chrome.ExtensionBackgroundPageURL(l.testExtID)
	testing.ContextLog(ctx, "Waiting for test API extension at ", bgURL)
	var err error
	if l.testExtConn, err = l.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL)); err != nil {
		return nil, err
	}
	l.testExtConn.Lock()

	// Ensure that we don't attempt to use the extension before its APIs are available: https://crbug.com/789313
	if err := l.testExtConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "test API extension is unavailable")
	}

	if err := l.testExtConn.Exec(ctx, "chrome.autotestPrivate.initializeEvents()"); err != nil {
		return nil, errors.Wrap(err, "failed to initialize test API events")
	}

	testing.ContextLog(ctx, "Test API extension is ready")
	return &chrome.TestConn{Conn: l.testExtConn}, nil
}
