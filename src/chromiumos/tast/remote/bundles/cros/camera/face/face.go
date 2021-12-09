// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package face provides utilities for face releated functions.
package face

import (
	"context"
	"path/filepath"
	"regexp"
	"strconv"

	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

// CheckRoiSupport checks the camera suppors region of interest control.
func CheckRoiSupport(ctx context.Context, s *testing.State) bool {
	d := s.DUT()
	facing := s.Param().(pb.Facing)
	out, err := d.Conn().CommandContext(ctx, "media_v4l2_test", "--list_usbcam").Output()
	if err != nil {
		s.Fatal(err, " failed to list usb cameras")
	}
	checkRoiSupport := false
	usbCameraRegexp := regexp.MustCompile(`/dev/video\d+`)
	for _, m := range usbCameraRegexp.FindAllStringSubmatch(string(out), -1) {
		device := m[0]
		out, err := d.Conn().CommandContext(ctx, "media_v4l2_test", "--gtest_filter=*GetRoiSupport*", "--device_path="+device).Output()
		if err != nil {
			s.Fatal(err, " failed to get roi support info")
		}
		r := regexp.MustCompile("Facing:(front|back):(1|0)")
		m2 := r.FindAllStringSubmatch(string(out), -1)
		s.Logf("Find device:%s facing:%s, roi support:%s", device, m2[0][1], m2[0][2])
		if facing == pb.Facing_FACING_BACK && m2[0][1] == "back" && m2[0][2] == "1" {
			checkRoiSupport = true
			break
		}
		if facing == pb.Facing_FACING_FRONT && m2[0][1] == "front" && m2[0][2] == "1" {
			checkRoiSupport = true
			break
		}
	}
	return checkRoiSupport
}

// GetFaceLumaValue gets the luma value of face.
func GetFaceLumaValue(ctx context.Context, s *testing.State, facing pb.Facing, enableFaceAe bool, outdir, sceneLog string) int64 {
	d := s.DUT()
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
		"--dump_path="+sceneLog).CombinedOutput()
	if err != nil {
		s.Fatal(err, "failed to run cros_camera_test to get luma value")
	}

	// Copy result scene log image.
	if err := linuxssh.GetFile(ctx, d.Conn(), sceneLog, filepath.Join(outdir, filepath.Base(sceneLog)), linuxssh.PreserveSymlinks); err != nil {
		s.Fatal(err, "failed to pull scene log file from DUT")
	}
	if err := d.Conn().CommandContext(ctx, "rm", sceneLog).Run(); err != nil {
		s.Fatal(err, "failed to clean up scene log file from DUT")
	}

	lumaRegexp := regexp.MustCompile(`Luma Value:(\d+)`)
	m := lumaRegexp.FindStringSubmatch(string(out))
	luma, err := strconv.ParseInt(m[1], 10, 32)
	if err != nil {
		s.Fatal(err, "failed to parse luma value %q", m[1])
	}

	return luma
}
