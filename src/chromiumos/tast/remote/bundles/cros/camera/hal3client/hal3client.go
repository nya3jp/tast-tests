// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package hal3client provides common test utility to run all hal3 remote
// tests.
package hal3client

import (
	"bytes"
	"context"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/testexec"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

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

// RunTest runs HAL3 remote test on DUT with specified facing camera.
func RunTest(ctx context.Context, s *testing.State, test pb.HAL3CameraTest, facing pb.Facing) {
	d := s.DUT()

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the HAL3 service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	hal3Client := pb.NewHAL3ServiceClient(cl.Conn)
	response, testErr := hal3Client.RunTest(ctx, &pb.RunTestRequest{
		Test:   test,
		Facing: facing,
	})
	if testErr != nil {
		s.Error("Remote test failed: ", testErr)
	}
	if err := extractOutdir(ctx, response, s.OutDir()); err != nil {
		s.Error("Failed to extract remote logs: ", err)
	}
}
