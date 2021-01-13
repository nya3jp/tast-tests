// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package firmware

import (
	"context"
	"io/ioutil"
	"strings"

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

// readUpdaterLogs reads the latest and previous fingerprint firmware updater logs.
func readUpdaterLogs(s *testing.State) (string, string) {
	latestData, err := ioutil.ReadFile(latestLog)
	if err != nil {
		s.Fatal("Failed to read latest updater log: ", err)
	}
	previousData, err := ioutil.ReadFile(previousLog)
	if err != nil {
		if strings.Contains(err.Error(), "no such file or directory") {
			// Previous log doesn't exist, this is the first boot.
			return strings.TrimSpace(string(latestData)), ""
		}
		s.Fatal("Failed to read previous updater log: ", err)
	}
	return strings.TrimSpace(string(latestData)), strings.TrimSpace(string(previousData))
}

func FpUpdaterSucceeded(ctx context.Context, s *testing.State) {
	latest, prev := readUpdaterLogs(s)

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
