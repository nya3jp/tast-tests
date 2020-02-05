// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package launcher implements a library used to setup and launch linux-chrome.
package launcher

import (
	"bytes"
	"context"
	"io/ioutil"
	"os"
	"strings"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
)

const binaryPath = LacrosTestPath + "/lacros_binary"

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

// LinuxChromePids returns the pids of all linux-chrome related processes,
// including chrome, nacl_helper, etc. The output is a new-line separated list
// of pids. It's returned as a bytes.Buffer to facilitate piping as input to
// another bash command.
func LinuxChromePids(ctx context.Context) bytes.Buffer {
	psCmd := testexec.CommandContext(ctx, "ps", "aux")
	grepCmd := testexec.CommandContext(ctx, "grep", binaryPath)
	awkCmd := testexec.CommandContext(ctx, "awk", "{print $2}")

	grepCmd.Cmd.Stdin, _ = psCmd.Cmd.StdoutPipe()
	awkCmd.Cmd.Stdin, _ = grepCmd.Cmd.StdoutPipe()

	var b bytes.Buffer
	awkCmd.Stdout = &b

	psCmd.Start()
	grepCmd.Start()
	awkCmd.Start()

	psCmd.Wait()
	grepCmd.Wait()
	awkCmd.Wait()

	return b
}

// LaunchLinuxChrome launches a fresh instance of linux-chrome.
func LaunchLinuxChrome(ctx context.Context, p PreData) (*linuxChrome, error) {
	// Kills all instances of linux-chrome and other related executables.
	// Ignore errors, since they can occur for several expected reasons.
	// e.g. the process to kill has died as a response to another process
	// being killed.
	b := LinuxChromePids(ctx)
	killCmd := testexec.CommandContext(ctx, "xargs", "kill", "-9")
	killCmd.Cmd.Stdin = &b
	killCmd.Run()

	// Create a new temporary directory for user data dir. We don't bother
	// clearing it on shutdown, since it's a subdirectory of the binary
	// path, which is cleared by pre.go. We need to use a new temporary
	// directory for each invocation so that successive calls to
	// LaunchLinuxChrome don't interfere with each other.
	userDataDir, err := ioutil.TempDir(binaryPath, "")
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
		"--long=en-US",                                              // Language
		"--breakpad-dump-location=" + binaryPath,                    // Specify location for breakpad dump files.
		"about:blank",                                               // Specify first tab to load.
	}

	l := &linuxChrome{}
	l.cmd = testexec.CommandContext(ctx, binaryPath+"/chrome", args...)
	l.cmd.Cmd.Env = append(os.Environ(), "XDG_RUNTIME_DIR=/run/chrome", "LD_LIBRARY_PATH="+binaryPath)
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
