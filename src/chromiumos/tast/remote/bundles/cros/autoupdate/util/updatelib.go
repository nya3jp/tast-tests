// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package util

import (
	"context"
	"fmt"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/remote/updateutil"
	"chromiumos/tast/testing"
)

const (
	cleanupTimeoutN2M    = 30 * time.Second
	initTimeoutN2M       = 3 * time.Minute
	preUpdateTimeoutN2M  = 3 * time.Minute
	postUpdateTimeoutN2M = 3 * time.Minute
	// TotalTestTime is the maximum time the test expected to take.
	TotalTestTime = cleanupTimeoutN2M + initTimeoutN2M + preUpdateTimeoutN2M + postUpdateTimeoutN2M + updateutil.UpdateTimeout
)

// Operations contains operations performed at various points of update sequence.
// Each operation has timeout 3 minutes, except cleanup, which is 30 seconds.
type Operations struct {
	PreUpdate    func(ctx context.Context) error
	PostUpdate   func(ctx context.Context) error
	PostRollback func(ctx context.Context) error
	CleanUp      func(ctx context.Context)
}

// NToMTest drives autoupdate and calls to client code providing callbacks.
// deltaM parameter specifies amount of milestones to rollback.
func NToMTest(ctx context.Context, dut *dut.DUT, outDir string, rpcHint *testing.RPCHint, ops *Operations, deltaM int) error {
	// Reserve time for deferred calls.
	ctxForCleanUp := ctx
	ctx, cancel := ctxutil.Shorten(ctx, cleanupTimeoutN2M)
	defer cancel()

	if ops.CleanUp != nil {
		defer ops.CleanUp(ctxForCleanUp)
	}

	// Limit the timeout for init steps.
	initCtx, cancel := context.WithTimeout(ctx, initTimeoutN2M)
	defer cancel()

	if ops.PreUpdate != nil {
		testing.ContextLog(ctx, "Running PreUpdate")
		if err := ops.PreUpdate(initCtx); err != nil {
			return errors.Wrap(err, "failed to run the PreUpdate operation")
		}
	}

	// Limit the timeout for the preparation steps.
	preCtx, cancel := context.WithTimeout(ctx, preUpdateTimeoutN2M)
	defer cancel()

	lsbContent := map[string]string{
		lsbrelease.Board:     "",
		lsbrelease.Version:   "",
		lsbrelease.Milestone: "",
	}

	err := updateutil.FillFromLSBRelease(preCtx, dut, rpcHint, lsbContent)
	if err != nil {
		return errors.Wrap(err, "failed to get all the required information from lsb-release")
	}

	board := lsbContent[lsbrelease.Board]
	originalVersion := lsbContent[lsbrelease.Version]

	milestoneN, err := strconv.Atoi(lsbContent[lsbrelease.Milestone])
	if err != nil {
		return errors.Wrapf(err, "failed to convert milestone to integer %s", lsbContent[lsbrelease.Milestone])
	}
	milestoneM := milestoneN - deltaM // Target milestone.

	// Find the latest stable release for milestone M.
	paygen, err := updateutil.LoadPaygenFromGS(preCtx)
	if err != nil {
		return errors.Wrap(err, "failed to load paygen data")
	}

	filtered := paygen.FilterBoardChannelDeltaType(board, "stable", "OMAHA").FilterMilestone(milestoneM)
	latest, err := filtered.FindLatest()
	if err != nil {
		return errors.Wrapf(err, "failed to find the latest stable release for milestone %d and board %s", milestoneM, board)
	}
	rollbackVersion := latest.ChromeOSVersion

	builderPath := fmt.Sprintf("%s-release/R%d-%s", board, milestoneM, rollbackVersion)

	// Update the DUT.
	testing.ContextLogf(ctx, "Starting update from %s to %s", originalVersion, rollbackVersion)
	if err := updateutil.UpdateFromGS(ctx, dut, outDir, rpcHint, builderPath); err != nil {
		return errors.Wrapf(err, "failed to update DUT to image for %q from GS", builderPath)
	}

	// Limit the timeout for the verification steps.
	postCtx, cancel := context.WithTimeout(ctx, postUpdateTimeoutN2M)
	defer cancel()

	// Reboot the DUT.
	testing.ContextLog(ctx, "Rebooting the DUT after the update")
	if err := dut.Reboot(postCtx); err != nil {
		return errors.Wrap(err, "failed to reboot the DUT after update")
	}

	// Check the image version.
	version, err := updateutil.ImageVersion(postCtx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to read DUT image version after the update")
	}
	testing.ContextLogf(ctx, "The DUT image version after the update is %s", version)
	if version != rollbackVersion {
		if version == originalVersion {
			// Rollback is not needed here, the test execution can stop.
			return errors.New("the image version did not change after the update")
		}
		return errors.Wrapf(err, "failed to update the image, image version after the update is incorrect; got %s, want %s", version, rollbackVersion)
	}

	if ops.PostUpdate != nil {
		testing.ContextLog(ctx, "Running PostUpdate")
		if err := ops.PostUpdate(postCtx); err != nil {
			return errors.Wrap(err, "failed to run the PostUpdate operation")
		}
	}

	// Restore original image version with rollback.
	testing.ContextLog(ctx, "Restoring the original device image")
	if err := dut.Conn().CommandContext(postCtx, "update_engine_client", "--rollback", "--nopowerwash", "--follow").Run(); err != nil {
		return errors.Wrap(err, "failed to rollback the DUT")
	}

	// Reboot the DUT.
	testing.ContextLog(ctx, "Rebooting the DUT after the rollback")
	if err := dut.Reboot(postCtx); err != nil {
		return errors.Wrap(err, "failed to reboot the DUT after rollback")
	}

	// Check the image version.
	version, err = updateutil.ImageVersion(postCtx, dut, rpcHint)
	if err != nil {
		return errors.Wrap(err, "failed to read DUT image version after the update")
	}
	testing.ContextLogf(ctx, "The DUT image version after the rollback is %s", version)
	if version != originalVersion {
		return errors.Errorf("image version is not the original after the restoration; got %s, want %s", version, originalVersion)
	}

	if ops.PostRollback != nil {
		testing.ContextLog(ctx, "Running PostRollback")
		if err := ops.PostRollback(postCtx); err != nil {
			return errors.Wrap(err, "failed to run the PostRollback operation")
		}
	}

	return nil
}
