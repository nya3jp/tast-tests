// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher implements a library used to setup and launch lacros-chrome.
package launcher

import (
	"context"
	"fmt"
	"io/ioutil"
	"os"
	"strings"
	"syscall"
	"time"

	"github.com/mafredri/cdp/protocol/target"
	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// BinaryPath is the root directory for lacros-chrome related binaries.
const BinaryPath = LacrosTestPath + "/lacros_binary"

// LacrosChrome contains all state associated with a lacros-chrome instance
// that has been launched. Must call Close() to release resources.
type LacrosChrome struct {
	Devsess       *cdputil.Session  // Debugging session for lacros-chrome
	cmd           *testexec.Cmd     // The command context used to start lacros-chrome.
	logAggregator *jslog.Aggregator // collects JS console output
	testExtID     string            // ID for test extension exposing APIs
	testExtConn   *chrome.Conn      // connection to test extension exposing APIs
}

// Close kills a launched instance of lacros-chrome.
func (l *LacrosChrome) Close(ctx context.Context) error {
	if l.Devsess != nil {
		l.Devsess.Close(ctx)
		l.Devsess = nil
	}
	if l.cmd != nil {
		if err := l.cmd.Cmd.Process.Kill(); err != nil {
			testing.ContextLog(ctx, "Failed to kill lacros-chrome: ", err)
		}
		l.cmd.Cmd.Wait()
		l.cmd = nil
	}
	if l.logAggregator != nil {
		l.logAggregator.Close()
		l.logAggregator = nil
	}
	if l.testExtConn != nil {
		l.testExtConn.Close()
		l.testExtConn = nil
	}
	killLacrosChrome(ctx)
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

// killLacrosChrome kills all binaries whose executable contains the base path
// to lacros-chrome.
func killLacrosChrome(ctx context.Context) {
	// Kills all instances of lacros-chrome and other related executables.
	pids, err := PidsFromPath(ctx, BinaryPath)
	if err != nil {
		testing.ContextLog(ctx, "Error finding pids for lacros-chrome: ", err)
	}
	for _, pid := range pids {
		// We ignore errors, since it's possible the process has
		// already been killed.
		unix.Kill(pid, syscall.SIGKILL)
	}
}

// LaunchLacrosChrome launches a fresh instance of lacros-chrome.
func LaunchLacrosChrome(ctx context.Context, p PreData) (*LacrosChrome, error) {
	killLacrosChrome(ctx)

	// Create a new temporary directory for user data dir. We don't bother
	// clearing it on shutdown, since it's a subdirectory of the binary
	// path, which is cleared by pre.go. We need to use a new temporary
	// directory for each invocation so that successive calls to
	// LaunchLacrosChrome don't interfere with each other.
	userDataDir, err := ioutil.TempDir(BinaryPath, "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp dir")
	}

	l := &LacrosChrome{testExtID: p.Chrome.TestExtID()}
	extList := strings.Join(p.Chrome.ExtDirs(), ",")
	args := []string{
		"--ozone-platform=wayland",                  // Use wayland to connect to exo wayland server.
		"--no-sandbox",                              // Disable sandbox for now
		"--remote-debugging-port=0",                 // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",      // Allow Chrome to use the Chrome Automation API.
		"--whitelisted-extension-id=" + l.testExtID, // Whitelists the test extension to access all Chrome APIs.
		"--load-extension=" + extList,               // Load extensions
		"--no-first-run",                            // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--user-data-dir=" + userDataDir,            // Specify a --user-data-dir, which holds on-disk state for Chrome.
		"--lang=en-US",                              // Language
		"--breakpad-dump-location=" + BinaryPath,    // Specify location for breakpad dump files.
		"--window-size=800,600",
		"--log-file=" + userDataDir + "/logfile", // Specify log file location for debugging.
		"--enable-logging",                       // This flag is necessary to ensure the log file is written.
		"--enable-gpu-rasterization",             // Enable GPU rasterization. This is necessary to enable OOP rasterization.
		"--enable-oop-rasterization",             // Enable OOP rasterization.
		"--disable-extensions-except=" + extList, // Disable extensions other than the Tast test extension.
		chrome.BlankURL,                          // Specify first tab to load.
	}

	l.cmd = testexec.CommandContext(ctx, BinaryPath+"/chrome", args...)
	l.cmd.Cmd.Env = append(os.Environ(), "EGL_PLATFORM=surfaceless", "XDG_RUNTIME_DIR=/run/chrome")
	testing.ContextLog(ctx, "Starting chrome: ", strings.Join(args, " "))
	if err := l.cmd.Cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to launch lacros-chrome")
	}

	// Wait for a window that matches what a lacros window looks like.
	if err := ash.WaitForCondition(ctx, p.TestAPIConn, func(w *ash.Window) bool {
		return w.IsVisible && strings.HasPrefix(w.Title, "about:blank") && strings.HasPrefix(w.Name, "ExoShellSurface")
	}, &testing.PollOptions{Timeout: 10 * time.Second}); err != nil {
		return nil, errors.Wrap(err, "failed to wait for lacros-chrome window to be visible")
	}

	debuggingPortPath := userDataDir + "/DevToolsActivePort"
	if l.Devsess, err = cdputil.NewSession(ctx, debuggingPortPath); err != nil {
		l.Close(ctx)
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}

	l.logAggregator = jslog.NewAggregator()

	return l, nil
}

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm.
func (l *LacrosChrome) NewConnForTarget(ctx context.Context, tm chrome.TargetMatcher) (*chrome.Conn, error) {
	t, err := l.Devsess.WaitForTarget(ctx, tm)
	if err != nil {
		return nil, err
	}

	return l.newConnInternal(ctx, t.TargetID, t.URL)
}

// NewConn creates a new Chrome renderer and returns a connection to it.
// If url is empty, an empty page (about:blank) is opened. Otherwise, the page
// from the specified URL is opened. You can assume that the page loading has
// been finished when this function returns.
func (l *LacrosChrome) NewConn(ctx context.Context, url string, opts ...cdputil.CreateTargetOption) (*chrome.Conn, error) {
	if url == "" {
		testing.ContextLog(ctx, "Creating new blank page")
	} else {
		testing.ContextLog(ctx, "Creating new page with URL ", url)
	}
	targetID, err := l.Devsess.CreateTarget(ctx, url, opts...)
	if err != nil {
		return nil, err
	}

	return l.newConnInternal(ctx, targetID, url)
}

func (l *LacrosChrome) newConnInternal(ctx context.Context, id target.ID, url string) (*chrome.Conn, error) {
	conn, err := chrome.NewConn(ctx, l.Devsess, id, l.logAggregator, url, func(err error) error { return err })
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

// TestAPIConn returns a new chrome.TestConn instance for the lacros browser.
func (l *LacrosChrome) TestAPIConn(ctx context.Context) (*chrome.TestConn, error) {
	if l.testExtConn != nil {
		return &chrome.TestConn{Conn: l.testExtConn}, nil
	}

	bgURL := chrome.ExtensionBackgroundPageURL(l.testExtID)
	testing.ContextLog(ctx, "Waiting for test API extension at ", bgURL)
	var err error
	if l.testExtConn, err = l.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL)); err != nil {
		return nil, err
	}

	// Ensure that we don't attempt to use the extension before its APIs are available: https://crbug.com/789313
	if err := l.testExtConn.WaitForExpr(ctx, `document.readyState === "complete"`); err != nil {
		return nil, errors.Wrap(err, "test API extension is unavailable")
	}

	if err := l.testExtConn.Eval(ctx, "chrome.autotestPrivate.initializeEvents()", nil); err != nil {
		return nil, errors.Wrap(err, "failed to initialize test API events")
	}

	testing.ContextLog(ctx, "Test API extension is ready")
	return &chrome.TestConn{Conn: l.testExtConn}, nil
}
