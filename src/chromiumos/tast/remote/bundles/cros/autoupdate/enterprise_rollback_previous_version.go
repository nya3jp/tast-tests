// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/rollbackutil"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/testing"
)

type testParam struct {
	previousVersionTarget int
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnterpriseRollbackPreviousVersion,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the enterprise rollback feature by rolling back to a previous release",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"crisguerrero@chromium.org",
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:autoupdate"},
		SoftwareDeps: []string{"reboot", "chrome", "auto_update_stable"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.RollbackService",
			"tast.cros.autoupdate.UpdateService",
			"tast.cros.hwsec.OwnershipService",
		},
		Timeout: updateutil.UpdateTimeout + 12*time.Minute,
		Params: []testing.Param{{
			Name: "rollback_1_version",
			Val: testParam{
				previousVersionTarget: 1,
			},
		}, {
			Name: "rollback_2_versions",
			Val: testParam{
				previousVersionTarget: 2,
			},
		}, {
			Name: "rollback_3_versions",
			Val: testParam{
				previousVersionTarget: 3,
			},
		}},
		Fixture: fixture.Autoupdate,
	})
}

// EnterpriseRollbackPreviousVersion does not use enrollment so any
// functionality that depend on the enrollment of the device should be not be
// added to this test.
func EnterpriseRollbackPreviousVersion(ctx context.Context, s *testing.State) {
	paygen := s.FixtValue().(updateutil.WithPaygen).Paygen()

	board, originalVersion, milestoneN, err := rollbackutil.GetDeviceInfo(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to get device information: ", err)
	}

	// The target milestone depends on the parameter of the test.
	param := s.Param().(testParam)
	milestoneM := milestoneN - param.previousVersionTarget // Target milestone.

	// Find the latest release for milestone M.
	filtered := paygen.FilterBoard(board).FilterDeltaType("OMAHA").FilterMilestone(milestoneM)
	latest, err := filtered.FindLatest()
	if err != nil {
		// Unreleased boards are filtered with auto_update_stable, so there should
		// be an available image.
		s.Fatalf("Failed to find the latest release for milestone %d and board %s: %v", milestoneM, board, err)
	}

	rollbackVersion := latest.ChromeOSVersion

	// Make sure to clear the TPM, go back to the original image, and remove all
	// remains that may be left by a faulty rollback.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 4*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if err := rollbackutil.CleanRollbackDataAndPowerwash(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to clean rollback data after test: ", err)
		}

		if err := rollbackutil.RestoreOriginalImage(ctx, s.DUT(), s.RPCHint(), originalVersion); err != nil {
			s.Error("Failed to restore original image after test: ", err)
		}
	}(cleanupCtx)

	if err := rollbackutil.SimulatePowerwash(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to simulate powerwash before test: ", err)
	}

	networksInfo, err := rollbackutil.ConfigureNetworks(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to configure networks: ", err)
	}

	sensitive, err := rollbackutil.SaveRollbackData(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to save rollback data: ", err)
	}

	s.Logf("Starting update from %s to %s", originalVersion, rollbackVersion)
	if err := rollbackutil.PrepareRollbackUpdate(ctx, s.DUT(), s.RPCHint(), s.OutDir(), board, milestoneM, rollbackVersion); err != nil {
		s.Fatal("Failed to prepare rollback update: ", err)
	}

	// Ineffective reset is expected because rollback initiates TPM ownership.
	s.Log("Simulating powerwash and rebooting the DUT to fake rollback")
	if err := rollbackutil.SimulatePowerwashAndReboot(ctx, s.DUT()); err != nil && !errors.Is(err, hwsec.ErrIneffectiveReset) {
		s.Fatal("Failed to simulate powerwash and reboot into rollback image: ", err)
	}

	if err := rollbackutil.VerifyImageAfterRollback(ctx, s.DUT(), s.RPCHint(), rollbackVersion, originalVersion); err != nil {
		s.Fatal("Failed to verify image after rollback: ", err)
	}

	// Check rollback data preservation.
	if err := rollbackutil.VerifyRollbackData(ctx, networksInfo, s.DUT(), s.RPCHint(), sensitive); err != nil {
		s.Fatal("Failed to verify rollback: ", err)
	}
}
