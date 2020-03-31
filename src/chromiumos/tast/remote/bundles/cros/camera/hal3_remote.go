// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"bytes"
	"context"
	"strings"

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
		Params:       testParams(),
	})
}

func testParams() []testing.Param {
	facingAttr := func(facing pb.Facing) string {
		if facing == pb.Facing_FACING_BACK {
			return "camerabox_facing_back"
		}
		return "camerabox_facing_front"
	}
	paramName := func(test pb.HAL3CameraTest, facing pb.Facing) string {
		noPrefixFacing := facing.String()[len("FACING_"):]
		return strings.ToLower(test.String() + "_" + noPrefixFacing)
	}

	var params []testing.Param
	for _, facing := range []pb.Facing{pb.Facing_FACING_BACK, pb.Facing_FACING_FRONT} {
		for _, value := range pb.HAL3CameraTest_value {
			test := pb.HAL3CameraTest(value)
			params = append(params, testing.Param{
				Name:      paramName(test, facing),
				ExtraAttr: []string{facingAttr(facing)},
				Val:       &pb.RunTestRequest{Test: test, Facing: facing},
			})
		}
	}

	return params
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
	if response == nil {
		return errors.New("no response from HAL3 service")
	}

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
