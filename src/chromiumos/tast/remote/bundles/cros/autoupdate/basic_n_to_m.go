// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/testing"
)

const (
	preUpdateTimeoutN2M  = 2 * time.Minute
	postUpdateTimeoutN2M = 3 * time.Minute
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         BasicNToM,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Example test for updating to an older version using Nebraska and test images",
		Contacts: []string{
			"gabormagda@google.com", // Test author
		},
		Attr:         []string{}, // Manual execution only.
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
		},
		Timeout: preUpdateTimeoutN2M + updateutil.UpdateTimeout + postUpdateTimeoutN2M,
	})
}

func BasicNToM(ctx context.Context, s *testing.State) {
	// Limit the timeout for the preparation steps.
	preCtx, cancel := context.WithTimeout(ctx, preUpdateTimeoutN2M)
	defer cancel()

	lsbContent := map[string]string{
		lsbrelease.Board:     "",
		lsbrelease.Version:   "",
		lsbrelease.Milestone: "",
	}

	err := updateutil.FillFromLSBRelease(preCtx, s.DUT(), s.RPCHint(), lsbContent)
	if err != nil {
		s.Fatal("Failed to get all the required information from lsb-release: ", err)
	}

	board := lsbContent[lsbrelease.Board]
	originalVersion := lsbContent[lsbrelease.Version]

	milestoneN, err := strconv.Atoi(lsbContent[lsbrelease.Milestone])
	if err != nil {
		s.Fatalf("Failed to convert milestone to integer %s: %v", lsbContent[lsbrelease.Milestone], err)
	}
	milestoneM := milestoneN - 3 // Target milestone.

	// Find the latest stable release for milestone M.
	paygen, err := updateutil.LoadPaygenFromGS(preCtx)
	if err != nil {
		s.Fatal("Failed to load paygen data: ", err)
	}

	filtered := paygen.FilterBoardChannelDeltaType(board, "stable", "OMAHA").FilterMilestone(milestoneM)
	latest, err := filtered.FindLatest()
	if err != nil {
		s.Fatalf("Failed to find the latest stable release for milestone %d and board %s: %v", milestoneM, board, err)
	}
	rollbackVersion := latest.ChromeOSVersion

	builderPath := fmt.Sprintf("%s-release/R%d-%s", board, milestoneM, rollbackVersion)

	// Update the DUT.
	s.Logf("Starting update from %s to %s", originalVersion, rollbackVersion)
	if err := updateutil.UpdateFromGS(ctx, s.DUT(), s.OutDir(), s.RPCHint(), builderPath); err != nil {
		s.Fatalf("Failed to update DUT to image for %q from GS: %v", builderPath, err)
	}

	// Limit the timeout for the verification steps.
	postCtx, cancel := context.WithTimeout(ctx, postUpdateTimeoutN2M)
	defer cancel()

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the update")
	if err := s.DUT().Reboot(postCtx); err != nil {
		s.Fatal("Failed to reboot the DUT after update: ", err)
	}

	// Check the image version.
	version, err := updateutil.ImageVersion(postCtx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Logf("The DUT image version after the update is %s", version)
	if version != rollbackVersion {
		if version == originalVersion {
			// Rollback is not needed here, the test execution can stop.
			s.Fatal("The image version did not change after the update")
		}
		s.Errorf("Unexpected image version after the update; got %s, want %s", version, rollbackVersion)
	}

	// Restore original image version with rollback.
	s.Log("Restoring the original device image")
	if err := s.DUT().Conn().CommandContext(postCtx, "update_engine_client", "--rollback", "--nopowerwash", "--follow").Run(); err != nil {
		s.Error("Failed to rollback the DUT: ", err)
	}

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the rollback")
	if err := s.DUT().Reboot(postCtx); err != nil {
		s.Fatal("Failed to reboot the DUT after rollback: ", err)
	}

	// Check the image version.
	version, err = updateutil.ImageVersion(postCtx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Logf("The DUT image version after the rollback is %s", version)
	if version != originalVersion {
		s.Errorf("Image version is not the original after the restoration; got %s, want %s", version, originalVersion)
	}
}
