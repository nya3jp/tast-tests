// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher implements a library used to setup and launch lacros-chrome.
package launcher

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"
	"syscall"
	"time"

	"android.googlesource.com/platform/external/perfetto/protos/perfetto/trace/github.com/google/perfetto/perfetto_proto"
	"github.com/mafredri/cdp/protocol/target"
	"github.com/shirou/gopsutil/process"
	"golang.org/x/sys/unix"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/chrome/lacros/faillog"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// LacrosUserDataDir is the directory that contains the user data of lacros.
const LacrosUserDataDir = "/home/chronos/user/lacros/"

// LacrosChrome contains all state associated with a lacros-chrome instance
// that has been launched. Must call Close() to release resources.
type LacrosChrome struct {
	lacrosPath  string // Root directory for lacros-chrome.
	userDataDir string // User data directory

	cmd  *testexec.Cmd // The command context used to start lacros-chrome.
	agg  *jslog.Aggregator
	sess *driver.Session // Debug session connected lacros-chrome.
}

// Browser returns a Browser instance.
func (l *LacrosChrome) Browser() *browser.Browser {
	return browser.New(l.sess)
}

// ConnectToLacrosChrome connects to a running lacros instance (e.g launched by the UI) and returns a LacrosChrome object that can be used to interact with it.
func ConnectToLacrosChrome(ctx context.Context, lacrosPath, userDataDir string) (l *LacrosChrome, retErr error) {
	debuggingPortPath := filepath.Join(userDataDir, "DevToolsActivePort")
	execPath := filepath.Join(lacrosPath, "chrome")

	agg := jslog.NewAggregator()
	defer func() {
		if retErr != nil {
			agg.Close()
		}
	}()

	sess, err := driver.NewSession(ctx, execPath, debuggingPortPath, cdputil.WaitPort, agg)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}
	return &LacrosChrome{
		lacrosPath:  lacrosPath,
		userDataDir: userDataDir,
		agg:         agg,
		sess:        sess,
	}, nil
}

// StartTracing starts trace events collection for the selected categories. Android
// categories must be prefixed with "disabled-by-default-android ", e.g. for the
// gfx category, use "disabled-by-default-android gfx", including the space.
// This must not be called after Close().
func (l *LacrosChrome) StartTracing(ctx context.Context, categories []string, opts ...cdputil.TraceOption) error {
	return l.sess.StartTracing(ctx, categories, opts...)
}

// StartSystemTracing starts trace events collection from the system tracing
// service using the marshaled binary protobuf trace config.
// Note: StopTracing should be called even if StartTracing returns an error.
// Sometimes, the request to start tracing reaches the browser process, but there
// is a timeout while waiting for the reply.
func (l *LacrosChrome) StartSystemTracing(ctx context.Context, perfettoConfig []byte) error {
	return l.sess.StartSystemTracing(ctx, perfettoConfig)
}

// StopTracing stops trace collection and returns the collected trace events.
// This must not be called after Close().
func (l *LacrosChrome) StopTracing(ctx context.Context) (*perfetto_proto.Trace, error) {
	return l.sess.StopTracing(ctx)
}

// Close kills a launched instance of lacros-chrome.
func (l *LacrosChrome) Close(ctx context.Context) error {
	if err := l.sess.Close(ctx); err != nil {
		testing.ContextLog(ctx, "Failed to close connection to lacros-chrome: ", err)
	}
	l.sess = nil
	l.agg.Close()
	l.agg = nil

	if l.cmd != nil {
		if err := l.cmd.Kill(); err != nil {
			testing.ContextLog(ctx, "Failed to kill lacros-chrome: ", err)
		}
		l.cmd.Wait()
		l.cmd = nil
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

// ExtensionArgs returns a list of args needed to pass to a lacros instance to enable the test extension.
func ExtensionArgs(extID, extList string) []string {
	return []string{
		"--remote-debugging-port=0",              // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",   // Allow Chrome to use the Chrome Automation API.
		"--allowlisted-extension-id=" + extID,    // Allowlists the test extension to access all Chrome APIs.
		"--load-extension=" + extList,            // Load extensions.
		"--disable-extensions-except=" + extList, // Disable extensions other than the Tast test extension.
	}
}

// LaunchLacrosChrome launches a fresh instance of lacros-chrome.
func LaunchLacrosChrome(ctx context.Context, f FixtValue) (*LacrosChrome, error) {
	return LaunchLacrosChromeWithURL(ctx, f, chrome.BlankURL)
}

// LaunchLacrosChromeWithURL launches a fresh instance of lacros-chrome having the given url.
func LaunchLacrosChromeWithURL(ctx context.Context, f FixtValue, url string) (*LacrosChrome, error) {
	succeeded := false
	defer faillog.SaveIf(ctx, f.LacrosPath(), func() bool { return !succeeded })

	if err := killLacrosChrome(ctx, f.LacrosPath()); err != nil {
		return nil, errors.Wrap(err, "failed to kill lacros-chrome")
	}

	// Create a new temporary directory for user data dir.
	// The directory will be wiped by fixture's Reset(), so if necessary
	// the log needs to be preserved within the test.
	// This creates new directory for each invocation to provide isolated environment.
	userDataDir, err := ioutil.TempDir(f.UserTmpDir(), "")
	if err != nil {
		return nil, errors.Wrapf(err, "failed to set up a user data dir: %v", userDataDir)
	}

	// Set user to chronos, since we run lacros as chronos.
	if err := os.Chown(userDataDir, int(sysutil.ChronosUID), int(sysutil.ChronosGID)); err != nil {
		return nil, errors.Wrap(err, "failed to chown user data dir")
	}

	extList := strings.Join(f.Chrome().DeprecatedExtDirs(), ",")
	args := []string{
		"--ozone-platform=wayland",                   // Use wayland to connect to exo wayland server.
		"--no-first-run",                             // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--user-data-dir=" + userDataDir,             // Specify a --user-data-dir, which holds on-disk state for Chrome.
		"--lang=en-US",                               // Language
		"--breakpad-dump-location=" + f.LacrosPath(), // Specify location for breakpad dump files.
		"--window-size=800,600",
		"--enable-logging=stderr",       // This flag is necessary to ensure the log file is written. Also include stderr - this matches the shelf launch case.
		"--enable-gpu-rasterization",    // Enable GPU rasterization. This is necessary to enable OOP rasterization.
		"--enable-oop-rasterization",    // Enable OOP rasterization.
		"--enable-webgl-image-chromium", // Enable WebGL image.
		"--use-cras",                    // Use CrAS.
		url,                             // Specify first tab to load.
	}
	args = append(args, ExtensionArgs(chrome.TestExtensionID, extList)...)
	args = append(args, f.Chrome().LacrosExtraArgs()...)

	cmd := testexec.CommandContext(ctx, "sudo", append([]string{"-E", "-u", "chronos",
		"/usr/local/bin/python3", "/usr/local/bin/mojo_connection_lacros_launcher.py",
		"-s", mojoSocketPath, filepath.Join(f.LacrosPath(), "chrome")}, args...)...)
	cmd.Env = append(os.Environ(), "EGL_PLATFORM=surfaceless", "XDG_RUNTIME_DIR=/run/chrome")

	if logFile, err := os.Create(faillog.LogFile(ctx)); err != nil {
		testing.ContextLog(ctx, "Failed to create lacros.log file: ", err)
	} else {
		defer logFile.Close()
		// Redirect both Stdout/Stderr to the same file.
		// Log lines may be mixed, but it should be ok, because it is for investigation.
		cmd.Stdout = logFile
		cmd.Stderr = logFile
	}

	testing.ContextLog(ctx, "Starting chrome: ", strings.Join(args, " "))
	if err := cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to launch lacros-chrome")
	}
	defer func() {
		if cmd == nil {
			return
		}
		if err := cmd.Kill(); err != nil {
			testing.ContextLog(ctx, "Failed to kill lacros-chrome: ", err)
		}
		cmd.Wait()
	}()

	// Wait for a window that matches what a lacros window looks like.
	if err := WaitForLacrosWindow(ctx, f.TestAPIConn(), ""); err != nil {
		return nil, err
	}
	l, err := ConnectToLacrosChrome(ctx, f.LacrosPath(), userDataDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}
	// Check if the URL passed in is open on the Lacros browser.
	if matches, err := l.FindTargets(ctx, chrome.MatchTargetURLPrefix(url)); err != nil {
		return nil, err
	} else if len(matches) == 0 {
		return nil, errors.Errorf("failed to find a matching URL: %v", url)
	}

	// Move cmd ownership to l, thus after this line terminating cmd wond't run.
	l.cmd = cmd
	cmd = nil

	succeeded = true
	return l, nil
}

// WaitForLacrosWindow waits for a Lacrow window to be open and have the title to be visible if it is specified as a param.
func WaitForLacrosWindow(ctx context.Context, tconn *chrome.TestConn, title string) error {
	if err := ash.WaitForCondition(ctx, tconn, func(w *ash.Window) bool {
		if !w.IsVisible {
			return false
		}
		if !strings.HasPrefix(w.Name, "ExoShellSurface") {
			return false
		}
		if len(title) > 0 {
			return strings.HasPrefix(w.Title, title)
		}
		return true
	}, &testing.PollOptions{Timeout: time.Minute, Interval: time.Second}); err != nil {
		return errors.Wrapf(err, "failed to wait for lacros-chrome window to be visible (title: %v)", title)
	}
	return nil
}

// NewConnForTarget iterates through all available targets and returns a connection to the
// first one that is matched by tm.
// This must not be called after Close().
func (l *LacrosChrome) NewConnForTarget(ctx context.Context, tm chrome.TargetMatcher) (*chrome.Conn, error) {
	return l.sess.NewConnForTarget(ctx, tm)
}

// FindTargets returns the info about Targets, which satisfies the given cond condition.
// This must not be called after Close().
func (l *LacrosChrome) FindTargets(ctx context.Context, tm chrome.TargetMatcher) ([]*chrome.Target, error) {
	return l.sess.FindTargets(ctx, tm)
}

// NewConn creates a new Chrome renderer and returns a connection to it.
// If url is empty, an empty page (about:blank) is opened. Otherwise, the page
// from the specified URL is opened. You can assume that the page loading has
// been finished when this function returns.
// This must not be called after Close().
func (l *LacrosChrome) NewConn(ctx context.Context, url string, opts ...cdputil.CreateTargetOption) (*chrome.Conn, error) {
	return l.sess.NewConn(ctx, url, opts...)
}

// TestAPIConn returns a new chrome.TestConn instance for the lacros browser.
// This must not be called after Close().
func (l *LacrosChrome) TestAPIConn(ctx context.Context) (*chrome.TestConn, error) {
	return l.sess.TestAPIConn(ctx)
}

// CloseAboutBlank finds all targets that are about:blank, closes them, then waits until they are gone.
// windowsExpectedClosed indicates how many windows that we expect to be closed from doing this operation.
// This takes *ash-chrome*'s TestConn as tconn, not the one provided by LacrosChrome.TestAPIConn.
func (l *LacrosChrome) CloseAboutBlank(ctx context.Context, tconn *chrome.TestConn, windowsExpectedClosed int) error {
	prevWindows, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		return err
	}

	targets, err := l.sess.FindTargets(ctx, driver.MatchTargetURL(chrome.BlankURL))
	if err != nil {
		return errors.Wrap(err, "failed to query for about:blank pages")
	}
	allPages, err := l.sess.FindTargets(ctx, func(t *target.Info) bool { return t.Type == "page" })
	if err != nil {
		return errors.Wrap(err, "failed to query for all pages")
	}

	for _, info := range targets {
		if err := l.sess.CloseTarget(ctx, info.TargetID); err != nil {
			return err
		}
	}
	return testing.Poll(ctx, func(ctx context.Context) error {
		// If we are closing all lacros targets, then lacros Chrome will exit. In that case, we won't be able to
		// communicate with it, so skip checking the targets. Since closing all lacros targets will close all
		// lacros windows, the window check below is necessary and sufficient.
		if len(targets) != len(allPages) {
			targets, err := l.sess.FindTargets(ctx, driver.MatchTargetURL(chrome.BlankURL))
			if err != nil {
				return testing.PollBreak(err)
			}
			if len(targets) != 0 {
				return errors.New("not all about:blank targets were closed")
			}
		}

		windows, err := ash.GetAllWindows(ctx, tconn)
		if err != nil {
			return testing.PollBreak(err)
		}
		if len(prevWindows)-len(windows) != windowsExpectedClosed {
			return errors.Errorf("expected %d windows to be closed, got %d closed",
				windowsExpectedClosed, len(prevWindows)-len(windows))
		}

		return nil
	}, &testing.PollOptions{Timeout: time.Minute})
}
