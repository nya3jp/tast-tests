// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/media/caps"
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
		SoftwareDeps: []string{"android_p", "arc_camera3", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.HAL3Service", "tast.cros.camerabox.ChartService"},
		Data:         []string{"scene.pdf"},
		Vars:         []string{"chart"},
		// For extra params, reference corresponding tests in:
		// src/platform/tast-tests/src/chromiumos/tast/local/bundles/cros/camera/hal3_*.go
		Params: []testing.Param{
			testing.Param{
				Name:      "device_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_DEVICE, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "device_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_DEVICE, Facing: pb.Facing_FACING_FRONT},
			},

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
				Name:      "module_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_MODULE, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "module_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_MODULE, Facing: pb.Facing_FACING_FRONT},
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

			testing.Param{
				Name:      "stream_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_STREAM, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "stream_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_STREAM, Facing: pb.Facing_FACING_FRONT},
			},
		},
	})
}

// sceneName is the name of chart for HAL3 default scene.
const sceneName = "scene.pdf"

func HAL3Remote(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Set up chart tablet.
	altChartTarget, ok := s.Var("chart")
	if !ok {
		testing.ContextLog(ctx, "No --var=chart= args present")
		altChartTarget = ""
	}

	chart, err := d.CameraboxChart(ctx, altChartTarget)
	if err != nil {
		s.Fatal("Failed to connect to the chart tablet: ", err)
	}

	cl, err := rpc.Dial(ctx, chart, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to rpc service on chart tablet: ", err)
	}
	defer cl.Close(ctx)

	chartClient := pb.NewChartServiceClient(cl.Conn)
	scenePath := s.DataPath(sceneName)
	content, err := ioutil.ReadFile(scenePath)
	if err != nil {
		s.Fatalf("Failed to read scene file %v: %v", scenePath, err)
	}
	if _, err := chartClient.Send(ctx, &pb.SendRequest{Name: sceneName, Content: content}); err != nil {
		s.Fatal("Remote call Send() failed: ", err)
	}
	if _, err := chartClient.Display(ctx, &pb.DisplayRequest{Name: sceneName}); err != nil {
		s.Fatal("Remote call Display() failed: ", err)
	}

	// Connect to the gRPC server on the DUT.
	cl, err = rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the HAL3 service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	// Reserve extra time for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	// Run remote test on DUT.
	hal3Client := pb.NewHAL3ServiceClient(cl.Conn)
	response, err := hal3Client.RunTest(ctx, s.Param().(*pb.RunTestRequest))
	if err != nil {
		s.Fatal("Remote call RunTest() failed: ", err)
	}
	// Assert err is nil then response should not be nil.
	defer func() {
		if err := d.Conn().Command("rm", "-r", response.OutPath).Start(cleanupCtx); err != nil {
			s.Errorf("Failed to cleanup remote output directory %q from DUT: %v", response.OutPath, err)
		}
	}()

	// Check test result.
	switch response.Result {
	case pb.TestResult_TEST_RESULT_PASSED:
	case pb.TestResult_TEST_RESULT_FAILED:
		s.Error("Remote test failed with error message:", response.Error)
	case pb.TestResult_TEST_RESULT_UNSET:
		s.Error("Remote test result is unset")
	}

	// Collect logs.
	if err := linuxssh.GetFile(ctx, d.Conn(), response.OutPath, s.OutDir()); err != nil {
		s.Errorf("Failed to get remote output directory %q from DUT: %v", response.OutPath, err)
	}
}
