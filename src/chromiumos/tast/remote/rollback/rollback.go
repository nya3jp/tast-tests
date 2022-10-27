// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package rollback

import (
	"context"
	"fmt"
	"strconv"
	"strings"

	"chromiumos/tast/common/hwsec"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/policyutil"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/rpc"
	aupb "chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/testing"
)

// DeviceInfo contains the information about the DUT before rollback.
type DeviceInfo struct {
	Board     string
	Version   string
	Milestone int
}

// SimulatePowerwash resets the TPM and system state.
func SimulatePowerwash(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) error {
	return policyutil.EnsureTPMAndSystemStateAreReset(ctx, dut, rpcHint)
}

// SimulatePowerwashAndReboot relies on the helper method to always reboot apart
// from doing the powerwash. If the internal functionality of
// EnsureTPMAndSystemStateAreResetRemote changes in the future and does not
// reboot the device, we will need to add reboot logic here for rollback to
// happen. See b/240541326.
func SimulatePowerwashAndReboot(ctx context.Context, dut *dut.DUT) error {
	return policyutil.EnsureTPMAndSystemStateAreResetRemote(ctx, dut)
}

// DUTInfo retrieves information about the device that is necessary for rollback
// from lsb-release: board, current version, and milestone.
func DUTInfo(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) (*DeviceInfo, error) {
	lsbContent := map[string]string{
		lsbrelease.Board:     "",
		lsbrelease.Version:   "",
		lsbrelease.Milestone: "",
	}
	err := updateutil.FillFromLSBRelease(ctx, dut, rpcHint, lsbContent)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get all the required information from lsb-release")
	}

	milestone, err := strconv.Atoi(lsbContent[lsbrelease.Milestone])
	if err != nil {
		return nil, errors.Wrapf(err, "failed to convert milestone to integer %s", lsbContent[lsbrelease.Milestone])
	}

	deviceInfo := &DeviceInfo{
		Board:     lsbContent[lsbrelease.Board],
		Version:   lsbContent[lsbrelease.Version],
		Milestone: milestone,
	}

	testing.ContextLogf(ctx, "Device information: %s (board) %s (version) %d (milestone)", deviceInfo.Board, deviceInfo.Version, deviceInfo.Milestone)
	return deviceInfo, nil
}

// ConfigureNetworks sets up the networks supported by rollback.
func ConfigureNetworks(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) ([]*aupb.NetworkInformation, error) {
	client, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer client.Close(ctx)

	// Configure networks to check preservation across rollback.
	rollbackService := aupb.NewRollbackServiceClient(client.Conn)
	response, err := rollbackService.SetUpNetworks(ctx, &aupb.SetUpNetworksRequest{})
	if err != nil {
		return nil, errors.Wrap(err, "failed to configure networks on client")
	}
	return response.Networks, nil
}

// SaveRollbackData manually goes through the different steps that take place
// during rollback. Returns sensitive data to check it is not accidentally
// logged anywhere during rollback.
func SaveRollbackData(ctx context.Context, dut *dut.DUT) (string, error) {
	// The .save_rollback_data flag would have been left by the update_engine on
	// an end-to-end rollback. We don't use update_engine. Place it manually.
	if err := dut.Conn().CommandContext(ctx, "touch", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
		return "", errors.Wrap(err, "failed to write rollback data save file")
	}

	// oobe_config_save would be started on shutdown but we need to fake
	// powerwash and call it now.
	if err := dut.Conn().CommandContext(ctx, "start", "oobe_config_save").Run(); err != nil {
		return "", errors.Wrap(err, "failed to run oobe_config_save")
	}

	// The following two commands would be done by clobber_state during powerwash
	// but the test does not powerwash.
	if err := dut.Conn().CommandContext(ctx, "sh", "-c", `cat /var/lib/oobe_config_save/data_for_pstore > /dev/pmsg0`).Run(); err != nil {
		return "", errors.Wrap(err, "failed to read rollback key")
	}
	// Adds a newline to pstore.
	if err := dut.Conn().CommandContext(ctx, "sh", "-c", `echo "" >> /dev/pmsg0`).Run(); err != nil {
		return "", errors.Wrap(err, "failed to add newline after rollback key")
	}

	sensitive, err := dut.Conn().CommandContext(ctx, "cat", "/var/lib/oobe_config_save/data_for_pstore").Output()
	if err != nil {
		return "", errors.Wrap(err, "failed to read sensitive data for pstore")
	}
	return string(sensitive), nil
}

// ToPreviousVersion prepares the device for rolling back, downloads the
// update to the target image, simulates a powerwash, and reboots into the new
// image.
func ToPreviousVersion(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, outDir, board string, targetMilestone int, rollbackVersion string) error {
	// Stopping ui early to prevent accidental reboots in the middle of TPM clear.
	// If you stop the ui while an update is pending, the device restarts.
	if err := dut.Conn().CommandContext(ctx, "stop", "ui").Run(); err != nil {
		return errors.Wrap(err, "failed to stop ui")
	}

	builderPath := fmt.Sprintf("%s-release/R%d-%s", board, targetMilestone, rollbackVersion)
	if err := updateutil.UpdateFromGS(ctx, dut, outDir, rpcHint, builderPath); err != nil {
		return errors.Wrapf(err, "failed to update DUT to image for %q from GS", builderPath)
	}

	// Ineffective reset is expected because rollback initiates TPM ownership.
	testing.ContextLog(ctx, "Simulating powerwash and rebooting the DUT to fake rollback")
	if err := SimulatePowerwashAndReboot(ctx, dut); err != nil && !errors.Is(err, hwsec.ErrIneffectiveReset) {
		return errors.Wrap(err, "failed to simulate powerwash and reboot into rollback image")
	}

	return nil
}

// CheckImageVersion verifies that the image after rollback has changed to
// the target version.
func CheckImageVersion(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, rollbackVersion, originalVersion string) error {
	// Check the image version.
	version, err := updateutil.ImageVersion(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to read DUT image version after the update")
	}
	testing.ContextLogf(ctx, "The DUT image version after the update is %s", version)
	if version != rollbackVersion {
		if version == originalVersion {
			return errors.New("the image version did not change after the update")
		}
		return errors.Errorf("unexpected image version after the update; got %s, want %s", version, rollbackVersion)
	}
	return nil
}

// VerifyRollbackData ensures that sensitive data was not accidentally logged
// during rollback but certain data, like network configuration, has been
// preserved.
func VerifyRollbackData(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, networks []*aupb.NetworkInformation, sensitive string) error {
	// Ensure that the sensitive data was not logged.
	logsAndCrashes := []string{"/var/log", "/var/spool/crash", "/home/chronos/crash", "/mnt/stateful_partition/unencrypted/preserve/crash", "/run/crash_reporter/crash"}
	for _, folder := range logsAndCrashes {
		err := dut.Conn().CommandContext(ctx, "grep", "-rq", sensitive, folder).Run()
		if err == nil {
			return errors.Errorf("sensitive data found by grep in folder %q", folder)
		}
	}

	client, err := rpc.Dial(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to connect to the RPC service on the DUT")
	}
	defer client.Close(ctx)

	rollbackService := aupb.NewRollbackServiceClient(client.Conn)
	response, err := rollbackService.VerifyRollback(ctx, &aupb.VerifyRollbackRequest{Networks: networks})
	if err != nil {
		return errors.Wrap(err, "failed to verify rollback on client")
	}
	if !response.Successful {
		return errors.Errorf("rollback was not successful: %s", response.VerificationDetails)
	}

	return nil
}

// ClearRollbackAndSystemData stops every process related to rollback,
// removes its flags and data created, and simulates powerwash.
func ClearRollbackAndSystemData(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint) error {
	dut.Conn().CommandContext(ctx, "stop", "oobe_config_save").Run()

	if err := dut.Conn().CommandContext(ctx, "rm", "-f", "/mnt/stateful_partition/.save_rollback_data").Run(); err != nil {
		return errors.Wrap(err, "failed to remove data save flag")
	}

	if err := dut.Conn().CommandContext(ctx, "rm", "-f", "/mnt/stateful_partition/rollback_data").Run(); err != nil {
		return errors.Wrap(err, "failed to remove rollback data")
	}

	if err := SimulatePowerwash(ctx, dut, rpcHint); err != nil {
		return errors.Wrap(err, "failed to simulate powerwash")
	}

	return nil
}

// RestoreOriginalImage makes sure that the device is left in the same state
// as it was before the test started by rolling back to the original version if
// needed.
func RestoreOriginalImage(ctx context.Context, dut *dut.DUT, rpcHint *testing.RPCHint, originalVersion string) error {
	// Check the image version. Roll back if it is not the original one or image
	// version can not be read.
	version, err := updateutil.ImageVersion(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to read DUT image version")
	}

	if version == originalVersion {
		testing.ContextLogf(ctx, "No need to restore original image because the version is %s already", version)
		return nil
	}

	testing.ContextLog(ctx, "Restoring the original device image")
	// Non-enterprise rollback is not restoring the image when the command is
	// called from this help method (b/241391509).
	// Use "rootdev -s" to get and log the root partition before and after.
	// The format is, for example: /dev/nvme0p3, /dev/mmcblk0p3, /dev/sda3.
	// This information is used for debugging, so it is not necessary to convert
	// the output into its corresponding index.
	if rootPartition, err := dut.Conn().CommandContext(ctx, "rootdev", "-s").Output(); err != nil {
		// We do not return error because it is for informational purpouses.
		testing.ContextLog(ctx, "Failed to read DUT root partition")
	} else {
		testing.ContextLogf(ctx, "Root partition before non-enterprise rollback is %s", strings.TrimSpace(string(rootPartition)))
	}

	if err := dut.Conn().CommandContext(ctx, "update_engine_client", "--rollback", "--nopowerwash", "--follow").Run(); err != nil {
		return errors.Wrap(err, "failed to rollback the DUT")
	}

	testing.ContextLog(ctx, "Rebooting the DUT after the non-enterprise rollback")
	if err := dut.Reboot(ctx); err != nil {
		return errors.Wrap(err, "failed to reboot the DUT after the non-enterprise rollback")
	}

	// Verify (non-enterprise) rollback.
	if rootPartition, err := dut.Conn().CommandContext(ctx, "rootdev", "-s").Output(); err != nil {
		// We do not return error because it is for informational purpouses.
		testing.ContextLog(ctx, "Failed to read DUT root partition")
	} else {
		testing.ContextLogf(ctx, "Root partition after non-enterprise rollback is %s", strings.TrimSpace(string(rootPartition)))
	}

	version, err = updateutil.ImageVersion(ctx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to read DUT image version after the non-enterprise rollback")
	}
	testing.ContextLogf(ctx, "The DUT image version after rollback is %s", version)
	if version != originalVersion {
		return errors.Errorf("Image version is not the original after the restoration; got %s, want %s", version, originalVersion)
	}

	return nil
}
