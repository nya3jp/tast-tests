// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package camerabox provides utilities for camerabox environment.
package camerabox

import (
	"context"
	"path"
	"strings"
	"time"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// LogTestScene takes a photo of test scene as log to debug scene related problem.
func LogTestScene(ctx context.Context, d *dut.DUT, facing pb.Facing, outdir string) (retErr error) {
	testing.ContextLog(ctx, "Capture scene log image")

	// Release camera unique resource from cros-camera temporarily for taking a picture of test scene.
	out, err := d.Command("status", "cros-camera").Output(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get initial state of cros-camera")
	}
	if strings.Contains(string(out), "start/running") {
		if err := d.Command("stop", "cros-camera").Run(ctx); err != nil {
			return errors.Wrap(err, "failed to stop cros-camera")
		}
		defer func() {
			if err := d.Command("start", "cros-camera").Run(ctx); err != nil {
				if retErr != nil {
					testing.ContextLog(ctx, "Failed to start cros-camera")
				} else {
					retErr = errors.Wrap(err, "failed to start cros-camera")
				}
			}
		}()
	}

	// Timeout for capturing scene image.
	captureCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	// Take a picture of test scene.
	facingArg := "back"
	if facing == pb.Facing_FACING_FRONT {
		facingArg = "front"
	}
	const sceneLog = "/tmp/scene.jpg"
	if err := d.Command(
		"sudo", "--user=arc-camera", "cros_camera_test",
		"--gtest_filter=Camera3StillCaptureTest/Camera3DumpSimpleStillCaptureTest.DumpCaptureResult/0",
		"--camera_facing="+facingArg,
		"--dump_still_capture_path="+sceneLog,
	).Run(captureCtx); err != nil {
		return errors.Wrap(err, "failed to run cros_camera_test to take a scene photo")
	}

	// Copy result scene log image.
	if err := linuxssh.GetFile(ctx, d.Conn(), sceneLog, path.Join(outdir, path.Base(sceneLog)), linuxssh.PreserveSymlinks); err != nil {
		return errors.Wrap(err, "failed to pull scene log file from DUT")
	}
	if err := d.Command("rm", sceneLog).Run(ctx); err != nil {
		return errors.Wrap(err, "failed to clean up scene log file from DUT")
	}
	return nil
}
