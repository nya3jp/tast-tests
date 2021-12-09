// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package face provides utilities for face related functions.
package face

import (
	"context"
	"fmt"
	"path/filepath"
	"regexp"
	"strconv"

	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/ssh"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// CheckRoiSupport checks the camera suppors region of interest control.
func CheckRoiSupport(ctx context.Context, d *dut.DUT, facing pb.Facing) (bool, error) {
	out, err := d.Conn().CommandContext(ctx, "media_v4l2_test", "--list_usbcam").Output(ssh.DumpLogOnError)
	if err != nil {
		return false, err
	}
	checkRoiSupport := false
	usbCameraRegexp := regexp.MustCompile(`/dev/video\d+`)
	for _, m := range usbCameraRegexp.FindAllStringSubmatch(string(out), -1) {
		device := m[0]
		out, err := d.Conn().CommandContext(ctx, "media_v4l2_test", "--gtest_filter=*GetRoiSupport*", "--device_path="+device).Output(ssh.DumpLogOnError)
		if err != nil {
			return false, err
		}
		r := regexp.MustCompile("Facing:(front|back):(1|0)")
		m2 := r.FindAllStringSubmatch(string(out), -1)
		testing.ContextLogf(ctx, "Find device:%s facing:%s, roi support:%s", device, m2[0][1], m2[0][2])
		if facing == pb.Facing_FACING_BACK && m2[0][1] == "back" && m2[0][2] == "1" {
			checkRoiSupport = true
			break
		}
		if facing == pb.Facing_FACING_FRONT && m2[0][1] == "front" && m2[0][2] == "1" {
			checkRoiSupport = true
			break
		}
	}
	return checkRoiSupport, nil
}

// GetFaceLumaValue gets the luma value of face. It copies sceneLog back and save under outdir.
func GetFaceLumaValue(ctx context.Context, d *dut.DUT, facing pb.Facing, enableFaceAe bool, outdir, sceneLogPrefix string) (int64, error) {
	sceneLog := fmt.Sprintf("/tmp/%s_%t.i420", sceneLogPrefix, enableFaceAe)
	testing.ContextLog(ctx, sceneLog)
	// Take a picture of test scene.
	facingArg := "back"
	if facing == pb.Facing_FACING_FRONT {
		facingArg = "front"
	}
	var testFilter string
	if enableFaceAe {
		testFilter = "--gtest_filter=*GetFaceLumaValueWithFaceAutoExposure*"
	} else {
		testFilter = "--gtest_filter=*GetFaceLumaValueWithoutFaceAutoExposure*"
	}
	out, err := d.Conn().CommandContext(ctx,
		"sudo", "--user=arc-camera", "cros_camera_test",
		testFilter,
		"--camera_facing="+facingArg,
		"--expected_num_faces=1",
		"--dump_path="+sceneLog).CombinedOutput(ssh.DumpLogOnError)
	if err != nil {
		return 0, errors.Wrap(err, string(out))
	}
	defer d.Conn().CommandContext(ctx, "rm", sceneLog).Run()

	// Copy result scene log image.
	if err := linuxssh.GetFile(ctx, d.Conn(), sceneLog, filepath.Join(outdir, filepath.Base(sceneLog)), linuxssh.DereferenceSymlinks); err != nil {
		return 0, err
	}

	lumaRegexp := regexp.MustCompile(`Luma Value:(\d+)`)
	m := lumaRegexp.FindStringSubmatch(string(out))
	luma, err := strconv.ParseInt(m[1], 10, 32)
	if err != nil {
		return 0, err
	}

	return luma, nil
}
