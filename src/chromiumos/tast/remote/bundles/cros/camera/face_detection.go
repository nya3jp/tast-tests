// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"regexp"

	"chromiumos/tast/common/camera/chart"
	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/remote/bundles/cros/camera/camerabox"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         FaceDetection,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Verifies face detection",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		SoftwareDeps: []string{"arc", "arc_camera3", caps.BuiltinUSBCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.HAL3Service"},
		Data:         []string{"its_scene2_c_20210708.png"},
		Vars:         []string{"chart"},
		Params: []testing.Param{
			{
				Name:      "back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       pb.Facing_FACING_BACK,
			},
			{
				Name:      "front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       pb.Facing_FACING_FRONT,
			},
		},
	})
}

func FaceDetection(ctx context.Context, s *testing.State) {
	d := s.DUT()
	facing := s.Param().(pb.Facing)

	// Check if this DUT has a fd enabled camera with the tested facing.
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

	if !checkRoiSupport {
		s.Log("Skip this DUT, because it doesn't support roi")
		return
	}

	var altAddr string
	if chartAddr, ok := s.Var("chart"); ok {
		altAddr = chartAddr
	}

	c, err := chart.New(ctx, s.DUT(), altAddr, s.DataPath("its_scene2_c_20210708.png"), s.OutDir())
	if err != nil {
		s.Fatal("Failed to prepare chart tablet: ", err)
	}
	defer c.Close(ctx, s.OutDir())

	if err := camerabox.LogTestScene(ctx, d, facing, s.OutDir()); err != nil {
		s.Error("Failed to take a photo of test scene: ", err)
	}

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the HAL3 service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Run remote test on DUT.
	hal3Client := pb.NewHAL3ServiceClient(cl.Conn)
	runTestRequest := &pb.RunTestRequest{Test: pb.HAL3CameraTest_FACE_DETECTION, Facing: facing, ExtendedParams: "3"}
	response, err := hal3Client.RunTest(ctx, runTestRequest)
	if err != nil {
		s.Fatal("Remote call RunTest() failed: ", err)
	}

	// Check test result.
	switch response.Result {
	case pb.TestResult_TEST_RESULT_PASSED:
	case pb.TestResult_TEST_RESULT_FAILED:
		s.Error("Remote test failed with error message:", response.Error)
	case pb.TestResult_TEST_RESULT_UNSET:
		s.Error("Remote test result is unset")
	}
}
