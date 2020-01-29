// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lacros

// Package chrome implements a library used to setup and launch linux-chrome.
import (
	"context"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

// Contains all state associated with a linux-chrome instance that has been launched.
// Must call Close() to release resources.
type LinuxChromeState struct {
	Devsess *cdputil.Session // Debugging session for linux-chrome
	cmd     *testexec.Cmd    // The command context used to start linux-chrome.
s *testing.State  // Testing state, needed by Close()
}

// Kills a launched instance of linux-chrome.
func (l *LinuxChromeState) Close() {
	if l.cmd != nil {
		if err := l.cmd.Cmd.Process.Kill(); err != nil {
			l.s.Error("Failed to kill linux-chrome: ", err)
		}
		l.cmd = nil
	}
	l.Devsess = nil
}

// Launches Linux Chrome.
func LaunchLinuxChrome(ctx context.Context, s *testing.State) (*LinuxChromeState, error) {
	p := s.PreValue().(PreData)
	binaryPath := LacrosTestPath + "/lacros_binary"

	// TODO: How do we kill a previously running instance of linux-chrome
	// if we don't know its PID?

	// Use a fixed user data dir, which makes debugging easier. We may wish
	// to switch to a temp dir in the future to avoid disk contamination in
	// the future.
	userDataDir := binaryPath + "/user_data"

	fields := []string{
		"--ozone-platform=wayland",                           // Use wayland to connect to exo wayland server.
		"--no-sandbox",                                       // Disable sandbox for now
		"--remote-debugging-port=0",                          // Let Chrome choose its own debugging port.
		"--enable-experimental-extension-apis",               // Allow Chrome to use the Chrome Automation API.
		"--whitelisted-extension-id=" + p.Chrome.TestExtID(),        // Whitelists the test extension to access all Chrome APIs.
		"--load-extension=" + strings.Join(p.Chrome.ExtDirs(), ","), // Load extensions
		"--no-first-run",                                     // Prevent showing up offer pages, e.g. google.com/chromebooks.
		"--user-data-dir=" + userDataDir,       // Specify a --user-data-dir, which holds on-disk state for Chrome.
		"--long=en-US",                                       // Language
		"--breakpad-dump-location=" + binaryPath,             // Specify location for breakpad dump files.
		"about:blank",                                        // Specify first tab to load.
	}

	l := new(LinuxChromeState)
	l.s = s
	l.cmd = testexec.CommandContext(ctx, binaryPath+"/chrome", fields...)
	l.cmd.Cmd.Env = os.Environ()
	l.cmd.Cmd.Env = append(l.cmd.Cmd.Env, "XDG_RUNTIME_DIR=/run/chrome")
	l.cmd.Cmd.Env = append(l.cmd.Cmd.Env, "LD_LIBRARY_PATH="+binaryPath)
	if err := l.cmd.Cmd.Start(); err != nil {
		l.cmd.DumpLog(ctx)
		l.Close()
		return nil, errors.Wrap(err, "failed to launch linux-chrome")
	}

	debuggingPortPath := userDataDir + "/DevToolsActivePort"
	var err error
	if l.Devsess, err = cdputil.NewSession(ctx, debuggingPortPath); err != nil {
		l.Close()
		return nil, errors.Wrap(err, "failed to connect to debugging port")
	}

	return l, nil
}
