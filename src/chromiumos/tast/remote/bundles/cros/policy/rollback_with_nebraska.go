// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"encoding/json"
	"fmt"
	"path"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/rpc"
	aupb "chromiumos/tast/services/cros/autoupdate"
	ppb "chromiumos/tast/services/cros/policy"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

type update struct {
	channel         string // channel is used for the ChromeOsReleaseChannel policy.
	subfolder       string // subfolder is the name of the payload copied to the stateful partition.
	versionPrefix   string // versionPrefix is used for the DeviceTargetVersionPrefix poliy.
	expectedVersion string
}

func init() {
	testing.AddTest(&testing.Test{
		Func: RollbackWithNebraska,
		Desc: "Example test for the enterprise rollback update using Nebraska and test images",
		Contacts: []string{
			"gabormagda@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{}, // Manual execution only.
		SoftwareDeps: []string{"reboot", "chrome"},
		ServiceDeps: []string{
			"tast.cros.autoupdate.NebraskaService",
			"tast.cros.autoupdate.UpdateService",
			"tast.cros.policy.PolicyService",
		},
		Timeout: 10 * time.Minute,
		Params: []testing.Param{
			{
				Name: "m93",
				Val: update{
					channel:         "stable-channel",
					subfolder:       "m93",
					versionPrefix:   "14092.",
					expectedVersion: "14092.35.0",
				},
			},
			{
				Name: "m92",
				Val: update{
					channel:         "stable-channel",
					subfolder:       "m92",
					versionPrefix:   "13982.",
					expectedVersion: "13982.82.0",
				},
			},
		},
	})
}

// Usage:
// Manual execution only, as the test images must be copied to DUT manually, as the test images are not public.
// The update payload and teh metadata file should be copied to a subfolder of /mnt/stateful_partition
// For example for subtest "m93" /mnt/stateful_partition/m93/
const (
	updateFolder = "/mnt/stateful_partition/"
)

func RollbackWithNebraska(ctx context.Context, s *testing.State) {
	params, ok := s.Param().(update)
	if !ok {
		s.Fatal("Failed to convert test parameters to the desired type")
	}

	originalVersion, err := imageVersion(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version before the update: ", err)
	}
	s.Log("The test is starting from image version ", originalVersion)

	// Reset TPM.
	if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
		s.Fatal("Failed to reset TPM: ", err)
	}
	resetTPM := false
	defer func(ctx context.Context) {
		if resetTPM {
			if err := policyutil.EnsureTPMAndSystemStateAreReset(ctx, s.DUT()); err != nil {
				s.Error("Failed to reset TPM after test: ", err)
			}
		}
	}(ctx)

	// Leave 2 minutes for restarts and restoration after the update.
	updateCtx, cancel := ctxutil.Shorten(ctx, 2*time.Minute)
	defer cancel()

	// Prepare and install the update:
	// * Enroll device.
	// * Set update related device policies.
	// * Configure and start Nebraska.
	// * Install the update.
	// * Save the logs.
	// Placed in a function, so defered cleanup steps are executed before device restart.
	func(ctx context.Context) {
		cleanupCtx := ctx
		ctx, cancel := ctxutil.Shorten(ctx, 1*time.Minute)
		defer cancel()

		// Connect to DUT.
		cl, err := rpc.Dial(ctx, s.DUT(), s.RPCHint(), "cros")
		if err != nil {
			s.Fatal("Failed to connect to the RPC service on the DUT: ", err)
		}
		defer cl.Close(cleanupCtx)

		// Create clients.
		nebraskaClient := aupb.NewNebraskaServiceClient(cl.Conn)
		policyClient := ppb.NewPolicyServiceClient(cl.Conn)
		updateClient := aupb.NewUpdateServiceClient(cl.Conn)

		// Enroll DUT.
		pJSON, err := json.Marshal(fakedms.NewPolicyBlob())
		if err != nil {
			s.Fatal("Failed to serialize policies: ", err)
		}

		resetTPM = true
		if _, err := policyClient.EnrollUsingChrome(ctx, &ppb.EnrollUsingChromeRequest{
			PolicyJson: pJSON,
		}); err != nil {
			s.Fatal("Failed to enroll using chrome: ", err)
		}
		defer policyClient.StopChromeAndFakeDMS(ctx, &empty.Empty{})

		// Set update policies.
		rollbackPolicies := []policy.Policy{
			&policy.DeviceTargetVersionPrefix{Val: params.versionPrefix},
			&policy.DeviceRollbackAllowedMilestones{Val: 4},
			// 3: Roll back and stay on target version if OS version is newer than target.
			// Try to carry over device-level configuration.
			&policy.DeviceRollbackToTargetVersion{Val: 3},
			&policy.ChromeOsReleaseChannel{Val: params.channel},
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

		if _, err := nebraskaClient.CreateTempDir(ctx, &empty.Empty{}); err != nil {
			s.Fatal("Failed to create temporary directory for Nebraska: ", err)
		}
		defer func(ctx context.Context) {
			if _, err := nebraskaClient.RemoveTempDir(ctx, &empty.Empty{}); err != nil {
				s.Error("Failed to remove the temporary directory: ", err)
			}
		}(cleanupCtx)

		// Start Nebraska (the fake Omaha service).
		dataDir := path.Join(updateFolder, params.subfolder)
		nebraska, err := nebraskaClient.Start(ctx, &aupb.StartRequest{
			Update: &aupb.Payload{
				Address:        "file://" + dataDir, // URL or file URL.
				MetadataFolder: dataDir,
			},
		})
		if err != nil {
			s.Fatal("Failed to start Nebraska: ", err)
		}
		defer func(ctx context.Context) {
			if err := linuxssh.GetFile(ctx, s.DUT().Conn(), nebraska.LogPath, filepath.Join(s.OutDir(), "nebraska.log"), linuxssh.DereferenceSymlinks); err != nil {
				s.Log("Failed to save Nebraska log: ", err)
			}
		}(cleanupCtx)
		defer func(ctx context.Context) {
			if _, err := nebraskaClient.Stop(ctx, &empty.Empty{}); err != nil {
				s.Error("Failed to stop Nebraska: ", err)
			}
		}(cleanupCtx)

		// Get the update log files even if the update fails.
		defer func(ctx context.Context) {
			if err := linuxssh.GetFile(ctx, s.DUT().Conn(), "/var/log/update_engine.log", filepath.Join(s.OutDir(), "update_engine.log"), linuxssh.DereferenceSymlinks); err != nil {
				s.Log("Failed to save update engine log: ", err)
			}
		}(cleanupCtx)

		// Do the update.
		if _, err := updateClient.CheckForUpdate(ctx, &aupb.UpdateRequest{
			OmahaUrl: fmt.Sprintf("http://127.0.0.1:%s/update?critical_update=True", nebraska.Port),
		}); err != nil {
			s.Fatal("Failed to check for updates: ", err)
		}
	}(updateCtx)

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the update")
	s.DUT().Reboot(ctx)
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT after rollback: ", err)
	}

	// Check the image version.
	version, err := imageVersion(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Logf("The DUT image version after the update is %s", version)
	if version != params.expectedVersion {
		if version == originalVersion {
			// Rollback is not needed here, the test execution can stop.
			s.Fatal("The image version did not change after the update")
		}
		s.Errorf("Unexpected image version after the update; got %s, want %s", version, params.expectedVersion)
	}

	// Restore original image version with rollback.
	if err := s.DUT().Conn().CommandContext(ctx, "update_engine_client", "--rollback", "--nopowerwash", "--follow").Run(); err != nil {
		s.Error("Failed to rollback the DUT: ", err)
	}

	// Reboot the DUT.
	s.Log("Rebooting the DUT after the rollback")
	s.DUT().Reboot(ctx)
	if err := s.DUT().Reboot(ctx); err != nil {
		s.Fatal("Failed to reboot the DUT after rollback: ", err)
	}

	// Check the image version again.
	version, err = imageVersion(ctx, s.DUT(), s.RPCHint())
	if err != nil {
		s.Fatal("Failed to read DUT image version after the update: ", err)
	}
	s.Log("The DUT image version after the rollback is ", version)
	if version != originalVersion {
		s.Errorf("Unexpected image version after rollback; got %s, want %s", version, originalVersion)
	}
}

func imageVersion(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) (string, error) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 20*time.Second)
	defer cancel()

	// Connect to DUT.
	cl, err := rpc.Dial(ctx, dut, rpcHint, "cros")
	if err != nil {
		return "", errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer cl.Close(cleanupCtx)

	// Check the new image version.
	updateClient := aupb.NewUpdateServiceClient(cl.Conn)
	response, err := updateClient.LSBReleaseContent(ctx, &empty.Empty{})
	if err != nil {
		return "", errors.Wrap(err, "failed to read lsb-release")
	}

	var lsb map[string]string
	if err := json.Unmarshal(response.ContentJson, &lsb); err != nil {
		return "", errors.Wrap(err, "failed to unmarshal lsb-relese content")
	}

	version, ok := lsb[lsbrelease.Version]
	if !ok {
		return "", errors.Wrap(err, "failed to get version from lsb-release content")
	}

	return version, nil
}
