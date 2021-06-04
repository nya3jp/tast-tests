// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"regexp"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/remote/bundles/cros/camera/camerabox"
	"chromiumos/tast/remote/bundles/cros/camera/chart"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

type faceDetectionParams struct {
	roiCapability  string
	runTestRequest *pb.RunTestRequest
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         FaceDetection,
		Desc:         "Verifies face detection",
		Contacts:     []string{"mojahsu@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		SoftwareDeps: []string{"arc", "arc_camera3", caps.BuiltinUSBCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.HAL3Service"},
		Data:         []string{"its_scene2_a_20210610.png"},
		Vars:         []string{"chart"},
		Params: []testing.Param{
			testing.Param{
				Name:      "back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val: faceDetectionParams{
					roiCapability:  "roi_usb_camera_back",
					runTestRequest: &pb.RunTestRequest{Test: pb.HAL3CameraTest_FACE_DETECTION, Facing: pb.Facing_FACING_BACK, ExpectedNumFaces: "3"},
				},
				Timeout: 15 * time.Minute,
			},
			testing.Param{
				Name:      "front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val: faceDetectionParams{
					roiCapability:  "roi_usb_camera_front",
					runTestRequest: &pb.RunTestRequest{Test: pb.HAL3CameraTest_FACE_DETECTION, Facing: pb.Facing_FACING_FRONT, ExpectedNumFaces: "3"},
				},
				Timeout: 15 * time.Minute,
			},
		},
	})
}

func FaceDetection(ctx context.Context, s *testing.State) {
	d := s.DUT()
	tp := s.Param().(faceDetectionParams)

	out, err := d.Command("avtest_label_detect").Output(ctx)
	if err != nil {
		s.Fatal(err, "failed to detect test labels")
	}
	checkRoiSupport := false
	detectedLabelRegexp := regexp.MustCompile(`(?m)^Detected label: (.*)$`)
	for _, m := range detectedLabelRegexp.FindAllStringSubmatch(string(out), -1) {
		label := strings.TrimSpace(m[1])
		if label == tp.roiCapability {
			checkRoiSupport = true
			break
		}
	}

	if !checkRoiSupport {
		s.Log("Skip this DUT, because it doesn't support ", tp.roiCapability)
		return
	}

	var altAddr string
	if chartAddr, ok := s.Var("chart"); ok {
		altAddr = chartAddr
	}

	c, err := chart.New(ctx, s.DUT(), altAddr, s.DataPath("its_scene2_a_20210610.png"), s.OutDir())
	if err != nil {
		s.Fatal("Failed to prepare chart tablet: ", err)
	}
	defer c.Close(ctx, s.OutDir())

	runTestRequest := tp.runTestRequest
	if err := camerabox.LogTestScene(ctx, d, runTestRequest.Facing, s.OutDir()); err != nil {
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
