// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"
	"encoding/json"
	"fmt"
	"strconv"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/rpc"
	aupb "chromiumos/tast/services/cros/autoupdate"
	ps "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         EnterpriseRollbackPreviousVersion,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests the enterprise rollback feature by rolling back to the previous release",
		Contacts: []string{
			"mpolzer@google.com", // Test author
			"chromeos-commercial-remote-management@google.com",
		},
		Attr:         []string{"group:autoupdate"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.RollbackService",
			"tast.cros.autoupdate.UpdateService",
			"tast.cros.policy.PolicyService",
		},
		Timeout: updateutil.UpdateTimeout + 12*time.Minute,
	})
}

func EnterpriseRollbackPreviousVersion(ctx context.Context, s *testing.State) {
	lsbContent := map[string]string{
		lsbrelease.Board:     "",
		lsbrelease.Version:   "",
		lsbrelease.Milestone: "",
	}

	err := updateutil.FillFromLSBRelease(ctx, s.DUT(), s.RPCHint(), lsbContent)
	if err != nil {
		s.Fatal("Failed to get all the required information from lsb-release: ", err)
	}

	board := lsbContent[lsbrelease.Board]
	originalVersion := lsbContent[lsbrelease.Version]

	milestoneN, err := strconv.Atoi(lsbContent[lsbrelease.Milestone])
	if err != nil {
		s.Fatalf("Failed to convert milestone to integer %s: %v", lsbContent[lsbrelease.Milestone], err)
	}
	milestoneM := milestoneN - 1 // Target milestone.

	// Find the latest release for milestone M.
	paygen, err := updateutil.LoadPaygenFromGS(ctx)
	if err != nil {
		s.Fatal("Failed to load paygen data: ", err)
	}

	filtered := paygen.FilterBoard(board).FilterDeltaType("OMAHA").FilterMilestone(milestoneM)
	latest, err := filtered.FindLatest()
	if err != nil {
		// For unreleased boards, e.g. -kernelnext it's expected that there is no
		// image available. Mark the test as successful and skip it.
		s.Logf("Skipping test; Failed to find the latest release for milestone %d and board %s: %v", milestoneM, board, err)
		return
	}

	// Make sure to clear the TPM, go back to the original image, and remove all
	// remains that may be left by a faulty rollback.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 4*time.Minute)
	defer cancel()
	defer func(ctx context.Context) {
		s.DUT().Conn().CommandContext(ctx, "stop", "oobe_config_save").Run()

		if err := s.DUT().Conn().CommandContext(ctx, "rm", "-f", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
			s.Error("Failed to remove data save flag: ", err)
		}

		if err := s.DUT().Conn().CommandContext(ctx, "rm", "-f", "/mnt/stateful_partition/rollback_data").Run(); err != nil {
			s.Error("Failed to remove rollback data: ", err)
		}

		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Error("Failed to reset TPM after test: ", err)
		}

		// Check the image version. Roll back if it's not the original one or image
		// version can't be read.
		version, err := updateutil.ImageVersion(ctx, s.DUT(), s.RPCHint())
		if err != nil {
			s.Error("Failed to read DUT image version: ", err)
		}

		if version != originalVersion {
			s.Log("Restoring the original device image")
			if err := s.DUT().Conn().CommandContext(ctx, "update_engine_client", "--rollback", "--nopowerwash", "--follow").Run(); err != nil {
				s.Error("Failed to rollback the DUT: ", err)
			}
		}

		s.Log("Rebooting the DUT after the rollback")
		if err := s.DUT().Reboot(ctx); err != nil {
			s.Fatal("Failed to reboot the DUT after rollback: ", err)
		}

		// Verify (non-enterprise) rollback.
		version, err = updateutil.ImageVersion(ctx, s.DUT(), s.RPCHint())
		if err != nil {
			s.Error("Failed to read DUT image version: ", err)
		}
		s.Logf("The DUT image version after the rollback is %s", version)
		if version != originalVersion {
			s.Errorf("Image version is not the original after the restoration; got %s, want %s", version, originalVersion)
		}
	}(cleanupCtx)

	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}

	// Enroll the device.
	client, err := rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	// RPC client will be closed before reboot to rollback.

	policyJSON, err := json.Marshal(fakedms.NewPolicyBlob())
	if err != nil {
		s.Fatal("Failed to serialize policies: ", err)
	}

	policyClient := ps.NewPolicyServiceClient(client.Conn)
	// Policy client will be closed after enrollment.

	if _, err := policyClient.EnrollUsingChrome(ctx, &ps.EnrollUsingChromeRequest{
		PolicyJson: policyJSON,
	}); err != nil {
		s.Fatal("Failed to enroll: ", err)
	}

	if _, err := policyClient.StopChromeAndFakeDMS(ctx, &empty.Empty{}); err != nil {
		s.Error("Failed to stop Chrome and Fake DMS: ", err)
	}

	// Configure PSK network to check preservation across rollback.
	rollbackService := aupb.NewRollbackServiceClient(client.Conn)
	response, err := rollbackService.SetUpPskNetwork(ctx, &empty.Empty{})
	if err != nil {
		s.Fatal("Failed to configure PSK network on client: ", err)
	}
	guid := response.Guid

	// The .save_rollback_data flag would have been left by the update_engine on
	// an end-to-end rollback. We don't use update_engine. Place it manually.
	if err := s.DUT().Conn().CommandContext(ctx, "touch",
		"/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
		s.Fatal("Failed to write rollback data save file: ", err)
	}

	// oobe_config_save would be started on shutdown but we need to fake
	// powerwash and call it now.
	if err := s.DUT().Conn().CommandContext(ctx, "start", "oobe_config_save").Run(); err != nil {
		s.Fatal("Failed to run oobe_config_save: ", err)
	}

	// The following two commands would be done by clobber_state during powerwash.
	if err := s.DUT().Conn().CommandContext(ctx, "sh", "-c",
		`cat /var/lib/oobe_config_save/data_for_pstore > /dev/pmsg0`).Run(); err != nil {
		s.Fatal("Failed to read rollback key: ", err)
	}
	// Adds a newline to pstore.
	if err := s.DUT().Conn().CommandContext(ctx, "sh", "-c", `echo >> /dev/pmsg0`).Run(); err != nil {
		s.Fatal("Failed to add newline after rollback key: ", err)
	}

	// Clean up the RPC client.
	if err := client.Close(ctx); err != nil {
		s.Error("Failed to close RPC client: ", err)
	}

	rollbackVersion := latest.ChromeOSVersion

	builderPath := fmt.Sprintf("%s-release/R%d-%s", board, milestoneM, rollbackVersion)

	// Stopping ui early to prevent accidental reboots in the middle of TPM clear.
	// If you stop the ui while an update is pending, the device restarts.
	if err := s.DUT().Conn().CommandContext(ctx, "stop", "ui").Run(); err != nil {
		s.Fatal("Failed to stop ui: ", err)
	}

	client, err = rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}

	s.Logf("Starting update from %s to %s", originalVersion, rollbackVersion)
	if err := updateutil.UpdateFromGS(ctx, s.DUT(), s.OutDir(), s.RPCHint(), builderPath); err != nil {
		s.Fatalf("Failed to update DUT to image for %q from GS: %v", builderPath, err)
	}

	// Reboot the DUT and reset TPM.
	// Ineffective reset is expected because rollback initiates TPM ownership.
	s.Log("Rebooting the DUT and resetting TPM to fake rollback")
	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil && !errors.Is(err, hwsec.ErrIneffectiveReset) {
		s.Fatal("Failed to reset TPM and reboot into rollback image: ", err)
	}

	// Check the image version.
	version, err := updateutil.ImageVersion(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Logf("The DUT image version after the update is %s", version)
	if version != rollbackVersion {
		if version == originalVersion {
			s.Fatal("The image version did not change after the update")
		}
		s.Errorf("Unexpected image version after the update; got %s, want %s", version, rollbackVersion)
	}

	// Check rollback data preservation.
	client, err = rpc.Dial(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
	}
	defer client.Close(ctx)

	rollbackService = aupb.NewRollbackServiceClient(client.Conn)
	verifyResponse, err := rollbackService.VerifyRollback(ctx, &aupb.VerifyRollbackRequest{Guid: guid})
	if err != nil {
		s.Fatal("Failed to verify rollback on client: ", err)
	}
	if !verifyResponse.Successful {
		s.Errorf("Rollback was not successful: %s", verifyResponse.VerificationDetails)
	} else {
		// On any milestone <100 Chrome was not ready to be tested yet, so it is not
		// possible to carry out all verification steps and they are skipped. The
		// verification is considered successful but if details are provided by the
		// service, they the should be logged.
		if verifyResponse.VerificationDetails != "" {
			s.Log(verifyResponse.VerificationDetails)
		}
	}
}
