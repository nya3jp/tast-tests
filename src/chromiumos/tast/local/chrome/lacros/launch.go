// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

import (
	"context"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/apps"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/internal/cdputil"
	"chromiumos/tast/local/chrome/internal/driver"
	"chromiumos/tast/local/chrome/jslog"
	"chromiumos/tast/local/chrome/lacros/lacrosfaillog"
	"chromiumos/tast/local/chrome/lacros/lacrosfixt"
	"chromiumos/tast/local/sysutil"
	"chromiumos/tast/testing"
)

// Setup runs lacros-chrome if indicated by the given browser.Type and returns some objects and interfaces
// useful in tests. If the browser.Type is Lacros, it will return a non-nil LacrosChrome instance or an error.
// If the browser.Type is Ash it will return a nil LacrosChrome instance.
func Setup(ctx context.Context, f interface{}, bt browser.Type) (*chrome.Chrome, *LacrosChrome, ash.ConnSource, error) {
	if _, ok := f.(chrome.HasChrome); !ok {
		return nil, nil, nil, errors.Errorf("unrecognized FixtValue type: %v", f)
	}
	cr := f.(chrome.HasChrome).Chrome()

	switch bt {
	case browser.TypeAsh:
		return cr, nil, cr, nil
	case browser.TypeLacros:
		f := f.(lacrosfixt.FixtValue)
		l, err := LaunchLacrosChrome(ctx, f)
		if err != nil {
			return nil, nil, nil, errors.Wrap(err, "failed to launch lacros-chrome")
		}
		return cr, l, l, nil
	default:
		return nil, nil, nil, errors.Errorf("unrecognized Chrome type %s", string(bt))
	}
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

// LaunchFromShelf launches lacros-chrome via shelf.
func LaunchFromShelf(ctx context.Context, tconn *chrome.TestConn, lacrosPath string) (*LacrosChrome, error) {
	const newTabTitle = "New Tab"

	testing.ContextLog(ctx, "Launch lacros via Shelf")
	if err := ash.LaunchAppFromShelf(ctx, tconn, apps.Lacros.Name, apps.Lacros.ID); err != nil {
		return nil, errors.Wrap(err, "failed to launch lacros via shelf")
	}

	testing.ContextLog(ctx, "Wait for Lacros window")
	if err := WaitForLacrosWindow(ctx, tconn, newTabTitle); err != nil {
		return nil, errors.Wrap(err, "failed to wait for lacros")
	}

	l, err := ConnectToLacrosChrome(ctx, lacrosPath, LacrosUserDataDir)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to lacros")
	}
	return l, nil
}

// LaunchLacrosChrome launches a fresh instance of lacros-chrome.
func LaunchLacrosChrome(ctx context.Context, f lacrosfixt.FixtValue) (*LacrosChrome, error) {
	return LaunchLacrosChromeWithURL(ctx, f, chrome.BlankURL)
}

// LaunchLacrosChromeWithURL launches a fresh instance of lacros-chrome having the given url.
func LaunchLacrosChromeWithURL(ctx context.Context, f lacrosfixt.FixtValue, url string) (*LacrosChrome, error) {
	succeeded := false
	defer lacrosfaillog.SaveIf(ctx, f.LacrosPath(), func() bool { return !succeeded })

	if err := killLacrosChrome(ctx, f.LacrosPath()); err != nil {
		return nil, errors.Wrap(err, "failed to kill lacros-chrome")
	}

	// Create a new temporary directory for user data dir. We don't bother
	// clearing it on shutdown, since it's a subdirectory of the binary
	// path, which is cleared by pre.go. We need to use a new temporary
	// directory for each invocation so that successive calls to
	// LaunchLacrosChrome don't interfere with each other.
	userDataDir, err := ioutil.TempDir(f.LacrosPath(), "")
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
	args = append(args, lacrosfixt.ExtensionArgs(chrome.TestExtensionID, extList)...)
	args = append(args, f.Chrome().LacrosExtraArgs()...)

	cmd := testexec.CommandContext(ctx, "sudo", append([]string{"-E", "-u", "chronos",
		"/usr/local/bin/python3", "/usr/local/bin/mojo_connection_lacros_launcher.py",
		"-s", lacrosfixt.MojoSocketPath, filepath.Join(f.LacrosPath(), "chrome")}, args...)...)
	cmd.Env = append(os.Environ(), "EGL_PLATFORM=surfaceless", "XDG_RUNTIME_DIR=/run/chrome")

	if logFile, err := os.Create(lacrosfaillog.LogFile(ctx)); err != nil {
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
