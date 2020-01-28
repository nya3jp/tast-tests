// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher implements a library used to setup and launch linux-chrome.
package launcher

import (
	"context"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

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
	return nil
}

// LaunchLinuxChrome launches a fresh instance of linux-chrome.
func LaunchLinuxChrome(ctx context.Context, p PreData) (*linuxChrome, error) {
	binaryPath := LacrosTestPath + "/lacros_binary"

	// TODO: How do we kill a previously running instance of
	// linux-chrome if we don't know its PID?
	// Use a fixed user data dir, which makes debugging easier. We
	// may wish to switch to a temp dir in the future to avoid disk
	// contamination in the future.
	userDataDir := binaryPath + "/user_data"

	args := []string{
		"--ozone-platform=wayland",                                  // Use wayland to connect to exo wayland server.
		"--no-sandbox",                                              // Disable sandbox for now
		"--remote-debugging-port=0",                                 // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",                      // Allow Chrome to use the Chrome Automation API.
		"--whitelisted-extension-id=" + p.Chrome.TestExtID(),        // Whitelists the test extension to access all Chrome APIs.
		"--load-extension=" + strings.Join(p.Chrome.ExtDirs(), ","), // Load extensions
		"--no-first-run",                                            // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--user-data-dir=" + userDataDir,                            // Specify a --user-data-dir, which holds on-disk state for Chrome.
		"--long=en-US",                                              // Language
		"--breakpad-dump-location=" + binaryPath,                    // Specify location for breakpad dump files.
		"about:blank",                                               // Specify first tab to load.
	}

	l := &linuxChrome{}
	l.cmd = testexec.CommandContext(ctx, binaryPath+"/chrome", args...)
	l.cmd.Cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR=/run/chrome", "LD_LIBRARY_PATH="+binaryPath)
	if err := l.cmd.Cmd.Start(); err != nil {
		return nil, errors.Wrap(err, "failed to launch linux-chrome")
	}

	debuggingPortPath := userDataDir + "/DevToolsActivePort"
	var err error
	if l.Devsess, err = cdputil.NewSession(ctx, debuggingPortPath); err != nil {
		l.Close(ctx)
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}

	return l, nil
}
