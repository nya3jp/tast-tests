// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"strings"

	"chromiumos/tast/local/firmware"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: FpUpdaterSucceeded,
		Desc: "Checks that the fingerprint firmware updater did not fail at boot",
		Contacts: []string{
			"tomhughes@chromium.org",
			"chromeos-fingerprint@google.com",
		},
		Attr:         []string{"group:mainline", "group:fingerprint-cq"},
		SoftwareDeps: []string{"biometrics_daemon"},
		HardwareDeps: hwdep.D(hwdep.Fingerprint()),
	})
}

const (
	successString = "The update was successful."
	// Differentiate between RO and RW failures, instead of using regex.
	roFailureString      = "Failed to update RO image, aborting."
	rwFailureString      = "Failed to update RW image, aborting."
	noUpdateString       = "Update was not necessary."
	noFirmwareFileString = "No firmware file on rootfs, exiting."
)

func FpUpdaterSucceeded(ctx context.Context, s *testing.State) {
	latest, prev, err := firmware.ReadFpUpdaterLogs()
	if err != nil {
		s.Fatal("Failed to read logs: ", err)
	}

	// After a successful update there's a reboot, so latest log should say no update needed.
	if strings.Contains(latest, noUpdateString) && strings.Contains(prev, successString) {
		return
	}

	// If both latest and previous log says no update, count as success.
	if strings.Contains(latest, noUpdateString) && strings.Contains(prev, noUpdateString) {
		return
	}

	// If latest log says no update and previous log does not exist, count as success.
	if strings.Contains(latest, noUpdateString) && prev == "" {
		return
	}

	// Everything else counts as failure.
	if strings.Contains(latest, roFailureString) || strings.Contains(prev, roFailureString) {
		s.Fatal("Failed to update RO")
	}

	if strings.Contains(latest, rwFailureString) || strings.Contains(prev, rwFailureString) {
		s.Fatal("Failed to update RW")
	}

	if strings.Contains(latest, noFirmwareFileString) || strings.Contains(prev, noFirmwareFileString) {
		s.Fatal("Failed to find firmware file on rootfs")
	}

	s.Fatal("Updater result unknown")
}
