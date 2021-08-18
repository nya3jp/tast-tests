// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	aupb "chromiumos/tast/services/cros/autoupdate"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: RollbackWithGaia,
		Desc: "Example test for the enterprise rollback update",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{}, // Manual execution only.
		VarDeps:      []string{"policy.RollbackWithGaia.confirm"},
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps:  []string{"tast.cros.policy.PolicyService", "tast.cros.autoupdate.UpdateService"},
		Timeout:      5 * time.Minute,
	})
}

func RollbackWithGaia(ctx context.Context, s *testing.State) {
	// Test should be started with -var=policy.RollbackWithGaia.confirm=ICanRollbackMyDUT
	// to avoid accidental execution of the test.
	if s.RequiredVar("policy.RollbackWithGaia.confirm") != "ICanRollbackMyDUT" {
		s.Log("You should only run this example test if you have manual access to your DUT")
		s.Log("After the update, you can restore the previous partition with the following command:")
		s.Log("\tupdate_engine_client --rollback --nopowerwash")

		s.Fatal("Failed to make sure it is an intentional test execution")
	}

	successfulUpdate := false

	func(ctx context.Context) {
		defer func(ctx context.Context) {
			if !successfulUpdate {
				if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
					s.Error("Failed to reset TPM after test: ", err)
				}
			}
		}(ctx)

		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 15*time.Second)
		defer cancel()

		// Reset TPM.
		if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
			s.Fatal("Failed to reset TPM: ", err)
		}

		// Connect to DUT.
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(cleanupCtx)

		// Create clients.
		policyClient := ppb.NewPolicyServiceClient(cl.Conn)
		updateClient := aupb.NewUpdateServiceClient(cl.Conn)

		// Enable the DUT to receive updates.
		originalContent, err := signBoardName(ctx, updateClient)
		if err != nil {
			s.Fatal("Failed to enable the DUT to recieve updates: ", err)
		}
		defer func(ctx context.Context, lsbContent []byte) {
			if _, err := updateClient.OverwriteStatefulLSBRelease(ctx, &aupb.LSBRelease{ContentJson: lsbContent}); err != nil {
				s.Log("Failed to restore lsb-release in the stateful partition: ", err)
			}
		}(cleanupCtx, originalContent)

		// Enroll DUT.
		pJSON, err := json.Marshal(fakedms.NewPolicyBlob())
		if err != nil {
			s.Fatal("Failed to serialize policies: ", err)
		}

		if _, err := policyClient.EnrollUsingChrome(ctx, &ppb.EnrollUsingChromeRequest{
			PolicyJson: pJSON,
		}); err != nil {
			s.Fatal("Failed to enroll using chrome: ", err)
		}
		defer policyClient.StopChromeAndFakeDMS(ctx, &empty.Empty{})

		// Set update policies.
		rollbackPolicies := []policy.Policy{
			// Note: the update will fail if the other partition already has the same image
			// that is selected below to rollback to.
			// &policy.DeviceTargetVersionPrefix{Val: "13982."}, // M92
			&policy.DeviceTargetVersionPrefix{Val: "13904."}, // M91
			&policy.DeviceRollbackAllowedMilestones{Val: 4},
			&policy.DeviceRollbackToTargetVersion{Val: 3}, // Roll back and stay on target version if OS version is newer than target. Try to carry over device-level configuration.
			&policy.ChromeOsReleaseChannel{Val: "stable-channel"},
			&policy.ChromeOsReleaseChannelDelegated{Val: false},
		}
		policyBlob := fakedms.NewPolicyBlob()
		policyBlob.AddPolicies(rollbackPolicies)

		pJSON, err = json.Marshal(policyBlob)
		if err != nil {
			s.Fatal("Failed to serialize policies: ", err)
		}
		if _, err := policyClient.UpdatePolicies(ctx, &ppb.UpdatePoliciesRequest{
			PolicyJson: pJSON,
		}); err != nil {
			s.Fatal("Failed to enroll using chrome: ", err)
		}

		// Get the update log files even if the update fails.
		defer func(ctx context.Context) {
			if err := linuxssh.GetFile(ctx, s.DUT().Conn(), "/var/log/update_engine.log", filepath.Join(s.OutDir(), "update_engine.log"), linuxssh.DereferenceSymlinks); err != nil {
				s.Log("Failed to save update engine log: ", err)
			}
		}(cleanupCtx)

		// Update DUT.
		if _, err := updateClient.CheckForUpdate(ctx, &aupb.UpdateRequest{}); err != nil {
			s.Fatal("Failed to check for updates: ", err)
		}

		successfulUpdate = true
	}(ctx)

	// Reboot the DUT.
	if successfulUpdate {
		s.Log("Update was successful, rebooting DUT")
		s.Log("Note: The DUT will remain enrolled after reboot")
		s.Log("Note: After the reboot the SSH connecton to the DUT is disabled,")
		s.Log("      manual restoration is required: update_engine_client --rollback --nopowerwash")

		rebootCtx, cancel := context.WithTimeout(ctx, 3*time.Second)
		defer cancel()
		// Restart in an independent process, so the SSH connection can be closed before the restart.
		s.DUT().Conn().CommandContext(rebootCtx, "nohup", "bash", "-c", "sleep 15; reboot;").Run() // Ignore the error.
	}
}

// signBoardName adds an entry to /mnt/stateful_partition/etc/lsb-release with a signed board name
// to enable the DUT to receive updates.
// Returns with the original content of /mnt/stateful_partition/etc/lsb-release so it can be restored after the update.
func signBoardName(ctx context.Context, client aupb.UpdateServiceClient) ([]byte, error) {
	// Get board name from /etc/lsb-release.
	response, err := client.LSBReleaseContent(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to read lsb-release")
	}

	var lsb map[string]string
	if err := json.Unmarshal(response.ContentJson, &lsb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal lsb-relese content")
	}

	board, ok := lsb[lsbrelease.Board]
	if !ok {
		return nil, errors.New("failed to determine DUT board")
	}
	signedBoardName := board + "-signed-mp-v3keys"

	// Get content of /mnt/stateful_partition/etc/lsb-release.
	response, err = client.StatefulLSBReleaseContent(ctx, &empty.Empty{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to read lsb-release on the stateful partition")
	}

	testing.ContextLogf(ctx, "Adding the %q board name to lsb-release in the stateful partition", signedBoardName)
	var statefulLsb map[string]string
	if err := json.Unmarshal(response.ContentJson, &statefulLsb); err != nil {
		return nil, errors.Wrap(err, "failed to unmarshal stateful lsb-relese content")
	}
	statefulLsb[lsbrelease.Board] = signedBoardName

	newStatefulLsbJSON, err := json.Marshal(statefulLsb)
	if err != nil {
		return nil, errors.Wrap(err, "failed to serialize stateful lsb-release content")
	}

	if _, err := client.OverwriteStatefulLSBRelease(ctx, &aupb.LSBRelease{ContentJson: newStatefulLsbJSON}); err != nil {
		return nil, errors.Wrap(err, "failed to overwrite lsb-release in the stateful partition")
	}

	return response.ContentJson, nil
}
