// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"path"
	"strings"
	"time"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/dut"
	"chromiumos/tast/errors"
	"chromiumos/tast/remote/bundles/cros/camera/pre"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/ssh/linuxssh"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3Remote,
		Desc:         "Verifies camera HAL3 interface function on remote DUT",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		SoftwareDeps: []string{"arc", "arc_camera3", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.HAL3Service"},
		Data:         []string{pre.DataChartScene().DataPath()},
		Vars:         []string{"chart"},
		Pre:          pre.DataChartScene(),
		// For extra params, reference corresponding tests in:
		// src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/camera/hal3_*.go
		Params: []testing.Param{
			testing.Param{
				Name:      "frame_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_FRAME, Facing: pb.Facing_FACING_BACK},
				Timeout:   15 * time.Minute,
			},
			testing.Param{
				Name:      "frame_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_FRAME, Facing: pb.Facing_FACING_FRONT},
				Timeout:   15 * time.Minute,
			},

			testing.Param{
				Name:      "perf_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_PERF, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "perf_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_PERF, Facing: pb.Facing_FACING_FRONT},
			},

			testing.Param{
				Name:      "preview_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_PREVIEW, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "preview_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_PREVIEW, Facing: pb.Facing_FACING_FRONT},
			},

			testing.Param{
				Name:      "recording_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_RECORDING, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "recording_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_RECORDING, Facing: pb.Facing_FACING_FRONT},
			},

			testing.Param{
				Name:      "still_capture_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_STILL_CAPTURE, Facing: pb.Facing_FACING_BACK},
				Timeout:   6 * time.Minute,
			},
			testing.Param{
				Name:      "still_capture_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_STILL_CAPTURE, Facing: pb.Facing_FACING_FRONT},
				Timeout:   6 * time.Minute,
			},
		},
	})
}

func HAL3Remote(ctx context.Context, s *testing.State) {
	d := s.DUT()
	runTestRequest := s.Param().(*pb.RunTestRequest)

	if err := logTestScene(ctx, d, runTestRequest.Facing, s.OutDir()); err != nil {
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

// logTestScene takes a photo of test scene as log to debug scene related problem.
func logTestScene(ctx context.Context, d *dut.DUT, facing pb.Facing, outdir string) (retErr error) {
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
