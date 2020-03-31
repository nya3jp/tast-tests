// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bytes"
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/media/caps"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         HAL3Remote,
		Desc:         "Verifies camera HAL3 interface function on remote DUT",
		Contacts:     []string{"inker@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		SoftwareDeps: []string{"android_p", "arc_camera3", caps.BuiltinCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.HAL3Service"},
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
			},
			testing.Param{
				Name:      "frame_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_FRAME, Facing: pb.Facing_FACING_FRONT},
			},

			testing.Param{
				Name:      "jda_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_JDA, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "jda_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_JDA, Facing: pb.Facing_FACING_FRONT},
			},

			testing.Param{
				Name:      "jea_back",
				ExtraAttr: []string{"camerabox_facing_back"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_JEA, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:      "jea_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_JEA, Facing: pb.Facing_FACING_FRONT},
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
			},
			testing.Param{
				Name:      "still_capture_front",
				ExtraAttr: []string{"camerabox_facing_front"},
				Val:       &pb.RunTestRequest{Test: pb.HAL3CameraTest_STILL_CAPTURE, Facing: pb.Facing_FACING_FRONT},
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

func HAL3Remote(ctx context.Context, s *testing.State) {
	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the HAL3 service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	hal3Client := pb.NewHAL3ServiceClient(cl.Conn)
	response, testErr := hal3Client.RunTest(ctx, s.Param().(*pb.RunTestRequest))
	if testErr != nil {
		s.Error("Remote test failed: ", testErr)
	}
	if err := extractOutdir(ctx, response, s.OutDir()); err != nil {
		s.Error("Failed to extract remote logs: ", err)
	}
}

// extractOutdir extracts compressed remote output directory to target directory.
func extractOutdir(ctx context.Context, response *pb.RunTestResponse, outDir string) error {
	if response.OutDir == nil {
		return errors.New("no output directory")
	}

	testing.ContextLog(ctx, "Extracting compressed remote output directory")
	cmd := testexec.CommandContext(ctx, "tar", "zxvC", outDir)
	cmd.Stdin = bytes.NewBuffer(response.OutDir)
	output, err := cmd.Output()
	testing.ContextLog(ctx, "Extraction finished: ", string(output))

	return err
}
