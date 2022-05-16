// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"regexp"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
)

const (
	// Autotest may be used on these routers too, and if it failed to clean up, we may be out of space in /tmp.

	// AutotestWorkdirGlob is the path that grabs all autotest outputs.
	AutotestWorkdirGlob = "/tmp/autotest-*"
	// WorkingDir is the tast-test's working directory.
	WorkingDir = "/tmp/tast-test/"
)

// RouterCloseContextDuration is a shorter context.Context duration is used for
// running things before Router.Close to reserve time for it to run.
const RouterCloseContextDuration = 5 * time.Second

// HostFileContentsMatch checks if the file at remoteFilePath on the remote
// host exists and that its contents match using the regex string matchRegex.
//
// Returns true if the file exists and its contents matches. Returns false
// with a nil error if the file does not exist.
func HostFileContentsMatch(ctx context.Context, host *ssh.Conn, remoteFilePath, matchRegex string) (bool, error) {
	// Verify that the file exists.
	fileExists, err := HostFileExists(ctx, host, remoteFilePath)
	if err != nil {
		return false, errors.Wrapf(err, "failed to check for the existence of file %q", remoteFilePath)
	}
	if !fileExists {
		return false, nil
	}

	// Verify that the file contents match.
	matcher, err := regexp.Compile(matchRegex)
	if err != nil {
		return false, errors.Wrapf(err, "failed to compile regex string %q", matchRegex)
	}
	fileContents, err := linuxssh.ReadFile(ctx, host, remoteFilePath)
	return matcher.Match(fileContents), nil
}

// HostFileExists checks the host for the file at remoteFilePath and returns
// true if remoteFilePath exists and is a regular file.
func HostFileExists(ctx context.Context, host *ssh.Conn, remoteFilePath string) (bool, error) {
	return HostTestPath(ctx, host, "-f", remoteFilePath)
}

// HostTestPath runs "test <testFlag> <remoteFilePath>" on the host and
// returns true if the test passes and false if the test fails.
func HostTestPath(ctx context.Context, host *ssh.Conn, testFlag, remotePath string) (bool, error) {
	if err := host.CommandContext(ctx, "test", testFlag, remotePath).Run(); err != nil {
		if err.Error() == "Process exited with status 1" {
			// Test was successfully evaluated and returned false.
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to run 'test %q %q' on remote host", testFlag, remotePath)
	}
	return true, nil
}
