// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/remote/rollback"
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
	deviceInfo, err := rollback.GetDeviceInfo(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to get device information: ", err)
	}

	// Make sure to clear the TPM, go back to the original image, and remove all
	// remains that may be left by a faulty rollback.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 4*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		if err := rollback.ClearRollbackAndSystemData(ctx, s.DUT(), s.RPCHint()); err != nil {
			s.Error("Failed to clean rollback data after test: ", err)
		}

		if err := rollback.RestoreOriginalImage(ctx, s.DUT(), s.RPCHint(), deviceInfo.Version); err != nil {
			s.Error("Failed to restore original image after test: ", err)
		}
	}(cleanupCtx)

	// The target milestone depends on the parameter of the test.
	// Before going through any setup for the test we want to be sure that there
	// is a release for the target milestone.
	param := s.Param().(testParam)
	targetMilestone := deviceInfo.Milestone - param.previousVersionTarget // Target milestone.

	// Find the latest release for milestone M.
	paygen := s.FixtValue().(updateutil.WithPaygen).Paygen()
	filtered := paygen.FilterBoard(deviceInfo.Board).FilterDeltaType("OMAHA").FilterMilestone(targetMilestone)
	latest, err := filtered.FindLatest()
	if err != nil {
		// Unreleased boards are filtered with auto_update_stable, so there should
		// be an available image.
		s.Fatalf("Failed to find the latest release for milestone %d and board %s: %v", targetMilestone, deviceInfo.Board, err)
	}

	// There is an image available for the target milestone; testing rollback.
	if err := rollback.SimulatePowerwash(ctx, s.DUT(), s.RPCHint()); err != nil {
		s.Fatal("Failed to simulate powerwash before test: ", err)
	}

	networksInfo, err := rollback.ConfigureNetworks(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to configure networks: ", err)
	}

	sensitive, err := rollback.SaveRollbackData(ctx, s.DUT())
	if err != nil {
		s.Fatal("Failed to save rollback data: ", err)
	}

	rollbackVersion := latest.ChromeOSVersion
	s.Logf("Starting update from %s to %s", deviceInfo.Version, rollbackVersion)
	if err := rollback.ToPreviousVersion(ctx, s.DUT(), s.RPCHint(), s.OutDir(), deviceInfo.Board, targetMilestone, rollbackVersion); err != nil {
		s.Fatal("Failed to rollback to previous version: ", err)
	}

	if err := rollback.CheckImageVersion(ctx, s.DUT(), s.RPCHint(), rollbackVersion, deviceInfo.Version); err != nil {
		s.Fatal("Failed to verify image after rollback: ", err)
	}

	// Check rollback data preservation.
	if err := rollback.VerifyRollbackData(ctx, s.DUT(), s.RPCHint(), networksInfo, sensitive); err != nil {
		s.Fatal("Failed to verify rollback: ", err)
	}
}
