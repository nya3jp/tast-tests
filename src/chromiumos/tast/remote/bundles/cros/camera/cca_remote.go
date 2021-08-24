// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"

	"chromiumos/tast/common/media/caps"
	"chromiumos/tast/remote/bundles/cros/camera/pre"
	"chromiumos/tast/rpc"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         CCARemote,
		Desc:         "Verifies CCA behavior on remote DUT",
		Contacts:     []string{"wtlee@chromium.org", "chromeos-camera-eng@google.com"},
		Attr:         []string{"group:camerabox"},
		SoftwareDeps: []string{"camera_app", "chrome", caps.BuiltinOrVividCamera},
		ServiceDeps:  []string{"tast.cros.camerabox.CCAService"},
		Data:         []string{pre.DocumentScene().DataPath(), "cca_ui.js"},
		Vars:         []string{"chart"},
		Pre:          pre.DocumentScene(),
		Params: []testing.Param{
			testing.Param{
				Name:              "document_scanning_back",
				ExtraAttr:         []string{"camerabox_facing_back"},
				ExtraSoftwareDeps: []string{"ondevice_document_scanner"},
				Val:               &pb.CCATestRequest{Test: pb.CCATest_DOCUMENT_SCANNING, Facing: pb.Facing_FACING_BACK},
			},
			testing.Param{
				Name:              "document_scanning_front",
				ExtraAttr:         []string{"camerabox_facing_front"},
				ExtraSoftwareDeps: []string{"ondevice_document_scanner"},
				Val:               &pb.CCATestRequest{Test: pb.CCATest_DOCUMENT_SCANNING, Facing: pb.Facing_FACING_FRONT},
			},
		},
	})
}

func CCARemote(ctx context.Context, s *testing.State) {
	d := s.DUT()
	testRequest := s.Param().(*pb.CCATestRequest)

	// Connect to the gRPC server on the DUT.
	cl, err := rpc.Dial(ctx, d, s.RPCHint(), "cros")
	if err != nil {
		s.Fatal("Failed to connect to the HAL3 service on the DUT: ", err)
	}
	defer cl.Close(ctx)

	scriptContent, err := ioutil.ReadFile(s.DataPath("cca_ui.js"))
	if err != nil {
		s.Fatal("Failed to load script: ", err)
	}
	testRequest.ScriptContents = [][]byte{scriptContent}

	// Run remote test on DUT.
	ccaClient := pb.NewCCAServiceClient(cl.Conn)
	testResponse, err := ccaClient.RunTest(ctx, testRequest)
	if err != nil {
		s.Fatal("Remote call RunTest() failed: ", err)
	}

	// Check test result.
	switch testResponse.Result {
	case pb.TestResult_TEST_RESULT_PASSED:
		s.Log("Remote test passed")
	case pb.TestResult_TEST_RESULT_FAILED:
		s.Error("Remote test failed with error message:", testResponse.Error)
	case pb.TestResult_TEST_RESULT_UNSET:
		s.Error("Remote test result is unset")
	}
}
