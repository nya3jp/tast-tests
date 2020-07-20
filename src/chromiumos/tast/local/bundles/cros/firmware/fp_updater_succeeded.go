// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"

	"chromiumos/tast/local/testexec"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpUpdaterSucceeded,
		Desc: "Checks that the fingerprint firmware updater did not fail at boot",
		Contacts: []string{
			"yichengli@chromium.org", // Test Author
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

const (
	latestLog     = "/var/log/biod/bio_fw_updater.LATEST"
	previousLog   = "/var/log/biod/bio_fw_updater.PREVIOUS"
	successString = "The update was successful."
	// Differentiate between RO and RW failures, instead of using regex.
	roFailureString      = "Failed to update RO image, aborting."
	rwFailureString      = "Failed to update RW image, aborting."
	noUpdateString       = "Update was not necessary."
	noFirmwareFileString = "No firmware file on rootfs, exiting."
)

func FpUpdaterSucceeded(ctx context.Context, s *testing.State) {
	// If either latest or previous log says success, the updater succeeded.
	cmd := testexec.CommandContext(ctx, "grep", "-q", successString, latestLog, previousLog)
	if err := cmd.Run(); err != nil {
		return
	}

	// If both latest and previous log says no update, count as success.
	cmd = testexec.CommandContext(ctx, "grep", "-q", noUpdateString, latestLog)
	if err := cmd.Run(); err != nil {
		cmd = testexec.CommandContext(ctx, "grep", "-q", noUpdateString, previousLog)
		if err := cmd.Run(); err != nil {
			return
		}
	}

	// Everything else counts as failure.
	cmd = testexec.CommandContext(ctx, "grep", "-q", roFailureString, latestLog, previousLog)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to update RO")
	}
	cmd = testexec.CommandContext(ctx, "grep", "-q", rwFailureString, latestLog, previousLog)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to update RW")
	}
	cmd = testexec.CommandContext(ctx, "grep", "-q", noFirmwareFileString, latestLog, previousLog)
	if err := cmd.Run(); err != nil {
		s.Fatal("Failed to find firmware file on rootfs")
	}
	s.Fatal("Updater result unknown")
}
