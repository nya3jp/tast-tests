// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package common

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/ssh"
)

const (
	// Autotest may be used on these routers too, and if it failed to clean up, we may be out of space in /tmp.

	// AutotestWorkdirGlob is the path that grabs all autotest outputs.
	AutotestWorkdirGlob = "/tmp/autotest-*"
	// WorkingDir is the tast-test's working directory.
	WorkingDir = "/tmp/tast-test/"
)

const (
	// NOTE: shill does not manage (i.e., run a dhcpcd on) the device with prefix "veth".
	// See kIgnoredDeviceNamePrefixes in http://cs/chromeos_public/src/platform2/shill/device_info.cc

	// VethPrefix is the prefix for the veth interface.
	VethPrefix = "vethA"
	// VethPeerPrefix is the prefix for the peer's veth interface.
	VethPeerPrefix = "vethB"
	// BridgePrefix is the prefix for the bridge interface.
	BridgePrefix = "tastbr"
)

// RouterCloseContextDuration is a shorter context.Context duration is used for
// running things before Router.Close to reserve time for it to run.
const RouterCloseContextDuration = 5 * time.Second

// HostFileContentsMatch checks if the file at remoteFilePath on the remote
// host that matches the grepMatchStr, as determined by the host's grep command.
//
// Returns true if the file exists and grep finds matches. Returns false
// with a nil error if the file does not exist. Returns false with a non-nil
// error if there was an issue checking for file existence or running grep with
// the file.
func HostFileContentsMatch(ctx context.Context, host *ssh.Conn, remoteFilePath, grepMatchStr string) (bool, error) {
	// Verify that the file exists
	if err := host.CommandContext(ctx, "test", "-f", remoteFilePath).Run(); err != nil {
		if err.Error() == "Process exited with status 1" {
			// File does not exist
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to check for the existence of file %q", remoteFilePath)
	}

	// Verify that grep can find results with the provided match string
	if err := host.CommandContext(ctx, "grep", "-q", grepMatchStr, remoteFilePath).Run(); err != nil {
		if err.Error() == "Process exited with status 1" {
			// Grep match failed
			return false, nil
		}
		return false, errors.Wrapf(err, "failed to run 'grep -q %q %q'", grepMatchStr, remoteFilePath)
	}
	return true, nil
}
