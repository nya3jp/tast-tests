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
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace"
	"github.com/mafredri/cdp/protocol/target"
	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// LacrosUserDataDir is the directory that contains the user data of lacros.
const LacrosUserDataDir = "/home/chronos/user/lacros/"

// LacrosChrome contains all state associated with a lacros-chrome instance
// that has been launched. Must call Close() to release resources.
type LacrosChrome struct {
	Devsess       *cdputil.Session  // Debugging session for lacros-chrome
	userDataDir   string            // User data directory
	cmd           *testexec.Cmd     // The command context used to start lacros-chrome.
	logAggregator *jslog.Aggregator // collects JS console output
	testExtConn   *chrome.Conn      // connection to test extension exposing APIs
	lacrosPath    string            // Root directory for lacros-chrome.
}

// ConnectToLacrosChrome connects to a running lacros instance (e.g launched by the UI) and returns a LacrosChrome object that can be used to interact with it.
func ConnectToLacrosChrome(ctx context.Context, lacrosPath, userDataDir string) (*LacrosChrome, error) {
	l := &LacrosChrome{lacrosPath: lacrosPath, userDataDir: userDataDir}
	debuggingPortPath := filepath.Join(userDataDir, "DevToolsActivePort")
	var err error
	if l.Devsess, err = cdputil.NewSession(ctx, debuggingPortPath, cdputil.WaitPort); err != nil {
		l.Close(ctx)
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}
	l.logAggregator = jslog.NewAggregator()
	return l, nil
}

// StartTracing starts trace events collection for the selected categories. Android
// categories must be prefixed with "disabled-by-default-android ", e.g. for the
// gfx category, use "disabled-by-default-android gfx", including the space.
func (l *LacrosChrome) StartTracing(ctx context.Context, categories []string, opts ...cdputil.TraceOption) error {
	return l.Devsess.StartTracing(ctx, categories, opts...)
}

// StopTracing stops trace collection and returns the collected trace events.
func (l *LacrosChrome) StopTracing(ctx context.Context) (*trace.Trace, error) {
	return l.Devsess.StopTracing(ctx)
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

	if err := killLacrosChrome(ctx, l.lacrosPath); err != nil {
		return errors.Wrap(err, "failed to kill lacros-chrome")
	}
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
func killLacrosChrome(ctx context.Context, lacrosPath string) error {
	if lacrosPath == "" {
		return errors.New("Path to lacros-chrome cannot be empty")
	}

	// Kills all instances of lacros-chrome and other related executables.
	pids, err := PidsFromPath(ctx, lacrosPath)
	if err != nil {
		return errors.Wrap(err, "error finding pids for lacros-chrome")
	}
	for _, pid := range pids {
		// We ignore errors, since it's possible the process has
		// already been killed.
		unix.Kill(pid, syscall.SIGKILL)
	}
	return nil
}

// extensionArgs returns a list of args needed to pass to a lacros instance to enable the test extension.
func extensionArgs(extID, extList string) []string {
	return []string{
		"--remote-debugging-port=0",              // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",   // Allow Chrome to use the Chrome Automation API.
		"--whitelisted-extension-id=" + extID,    // Whitelists the test extension to access all Chrome APIs.
		"--load-extension=" + extList,            // Load extensions.
		"--disable-extensions-except=" + extList, // Disable extensions other than the Tast test extension.
	}
}

// EnsureLacrosChrome ensures that the lacros binary is extracted.
// Currently, callers need to call this if they do not call LaunchLacrosChrome, but otherwise
// try to run lacros chrome.
func EnsureLacrosChrome(ctx context.Context, f FixtData, artifactPath string) error {
	if f.Mode != PreExist {
		return nil
	}

	// TODO(crbug.com/1127165): Move this to the fixture when we can use Data in fixtures.
	_, err := os.Stat(f.LacrosPath)
	if os.IsNotExist(err) {
		testing.ContextLog(ctx, "Extracting lacros binary")
		tarCmd := testexec.CommandContext(ctx, "tar", "-xvf", artifactPath, "-C", lacrosTestPath)
		if err := tarCmd.Run(testexec.DumpLogOnError); err != nil {
			return errors.Wrap(err, "failed to untar test artifacts")
		}

		if err := os.Chmod(f.LacrosPath, 0777); err != nil {
			return errors.Wrap(err, "failed to change permissions of the binary root dir path")
		}
	} else {
		return err
	}
	return nil
}

// LaunchLacrosChrome launches a fresh instance of lacros-chrome.
// TODO(crbug.com/1127165): Remove the artifactPath argument when we can use Data in fixtures.
func LaunchLacrosChrome(ctx context.Context, f FixtData, artifactPath string) (*LacrosChrome, error) {
	if err := EnsureLacrosChrome(ctx, f, artifactPath); err != nil {
		return nil, err
	}

	if err := killLacrosChrome(ctx, f.LacrosPath); err != nil {
		return nil, errors.Wrap(err, "failed to kill lacros-chrome")
	}

	// Create a new temporary directory for user data dir. We don't bother
	// clearing it on shutdown, since it's a subdirectory of the binary
	// path, which is cleared by pre.go. We need to use a new temporary
	// directory for each invocation so that successive calls to
	// LaunchLacrosChrome don't interfere with each other.
	userDataDir, err := ioutil.TempDir(f.LacrosPath, "")
	if err != nil {
		// Fall back to create it under /tmp in case rootfs-lacros is used.
		if userDataDir, err = ioutil.TempDir("/tmp", "lacros"); err != nil {
			return nil, errors.Wrapf(err, "failed to set up a user data dir: %v", userDataDir)
		}
	}

	// Set user to chronos, since we run lacros as chronos.
	if err := os.Chown(userDataDir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return nil, errors.Wrap(err, "failed to chown user data dir")
	}

	l := &LacrosChrome{lacrosPath: f.LacrosPath, userDataDir: userDataDir}
	extList := strings.Join(f.Chrome.DeprecatedExtDirs(), ",")
	args := []string{
		"--ozone-platform=wayland",                 // Use wayland to connect to exo wayland server.
		"--no-sandbox",                             // Disable sandbox for now
		"--no-first-run",                           // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--user-data-dir=" + l.userDataDir,         // Specify a --user-data-dir, which holds on-disk state for Chrome.
		"--lang=en-US",                             // Language
		"--breakpad-dump-location=" + f.LacrosPath, // Specify location for breakpad dump files.
		"--window-size=800,600",
		"--log-file=" + l.LogFile(),                  // Specify log file location for debugging.
		"--enable-logging",                           // This flag is necessary to ensure the log file is written.
		"--enable-gpu-rasterization",                 // Enable GPU rasterization. This is necessary to enable OOP rasterization.
		"--enable-oop-rasterization",                 // Enable OOP rasterization.
		"--enable-webgl-image-chromium",              // Enable WebGL image.
		"--autoplay-policy=no-user-gesture-required", // Allow media autoplay.
		"--use-cras",                                 // Use CrAS.
		chrome.BlankURL,                              // Specify first tab to load.
	}
	args = append(args, extensionArgs(chrome.TestExtensionID, extList)...)
	args = append(args, f.Chrome.LacrosExtraArgs()...)

	l.cmd = testexec.CommandContext(ctx, "/usr/local/bin/python3", append([]string{"/usr/local/bin/mojo_connection_lacros_launcher.py",
		"-s", mojoSocketPath, filepath.Join(f.LacrosPath, "chrome")}, args...)...)
	l.cmd.Cmd.Env = append(os.Environ(), "EGL_PLATFORM=surfaceless", "XDG_RUNTIME_DIR=/run/chrome")

	if out, ok := testing.ContextOutDir(ctx); !ok {
		testing.ContextLog(ctx, "OutDir not found: ", err)
	} else if logFile, err := os.Create(filepath.Join(out, "lacros.log")); err != nil {
		testing.ContextLog(ctx, "Failed to create lacros.log file: ", err)
	} else {
		defer logFile.Close()
		// Redirect both Stdout/Stderr to the same file.
		// Log lines may be mixed, but it should be ok, because it is for investigation.
		l.cmd.Stdout = logFile
		l.cmd.Stderr = logFile
	}

	testing.ContextLog(ctx, "Starting chrome: ", strings.Join(args, " "))
	if err := l.cmd.Cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to launch lacros-chrome")
	}

	// Wait for a window that matches what a lacros window looks like.
	if err := WaitForLacrosWindow(ctx, f.TestAPIConn, "about:blank"); err != nil {
		return nil, err
	}

	debuggingPortPath := filepath.Join(l.userDataDir, "DevToolsActivePort")
	if l.Devsess, err = cdputil.NewSession(ctx, debuggingPortPath, cdputil.WaitPort); err != nil {
		l.Close(ctx)
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}

	l.logAggregator = jslog.NewAggregator()

	return l, nil
}

// LogFile returns a path to a file containing Lacros's log.
func (l *LacrosChrome) LogFile() string {
	return filepath.Join(l.userDataDir, "logfile")
}

// WaitForLacrosWindow waits for a Lacrow window with the specified title to be visibe.
func WaitForLacrosWindow(ctx context.Context, tconn *chrome.TestConn, title string) error {
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		return w.IsVisible && strings.HasPrefix(w.Title, title) && strings.HasPrefix(w.Name, "ExoShellSurface")
	}, &testing.PollOptions{Timeout: time.Minute}); err != nil {
		return errors.Wrap(err, "failed to wait for lacros-chrome window to be visible")
	}
	return nil
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

// FindTargets returns the info about Targets, which satisfies the given cond condition.
func (l *LacrosChrome) FindTargets(ctx context.Context, tm chrome.TargetMatcher) ([]*chrome.Target, error) {
	return l.Devsess.FindTargets(ctx, tm)
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
	conn, err := chrome.DeprecatedNewConn(ctx, l.Devsess, id, l.logAggregator, url, func(err error) error { return err })
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

	bgURL := chrome.ExtensionBackgroundPageURL(chrome.TestExtensionID)
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
