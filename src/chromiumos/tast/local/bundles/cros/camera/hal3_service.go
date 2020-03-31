// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"io/ioutil"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/testexec"
	cameraboxpb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			cameraboxpb.RegisterHAL3ServiceServer(srv, &HAL3Service{s: s})
		},
	})
}

type HAL3Service struct {
	s *testing.ServiceState
}

type getTestConfig = func(outDir string) hal3.TestConfig

var getTestConfigMap = map[cameraboxpb.HAL3CameraTest]getTestConfig{
	cameraboxpb.HAL3CameraTest_DEVICE:        hal3.DeviceTestConfig,
	cameraboxpb.HAL3CameraTest_FRAME:         hal3.FrameTestConfig,
	cameraboxpb.HAL3CameraTest_JDA:           hal3.JDATestConfig,
	cameraboxpb.HAL3CameraTest_JEA:           hal3.JEATestConfig,
	cameraboxpb.HAL3CameraTest_MODULE:        hal3.ModuleTestConfig,
	cameraboxpb.HAL3CameraTest_PERF:          hal3.PerfTestConfig,
	cameraboxpb.HAL3CameraTest_PREVIEW:       hal3.PreviewTestConfig,
	cameraboxpb.HAL3CameraTest_RECORDING:     hal3.RecordingTestConfig,
	cameraboxpb.HAL3CameraTest_STILL_CAPTURE: hal3.StillCaptureTestConfig,
	cameraboxpb.HAL3CameraTest_STREAM:        hal3.StreamTestConfig,
}

func (c *HAL3Service) RunTest(ctx context.Context, req *cameraboxpb.RunTestRequest) (*cameraboxpb.RunTestResponse, error) {
	outDir, err := ioutil.TempDir("", "outdir")
	if err != nil {
		return &cameraboxpb.RunTestResponse{}, errors.Wrap(err, "failed to create remote output directory")
	}

	getTestConfig, ok := getTestConfigMap[req.Test]
	if !ok {
		return &cameraboxpb.RunTestResponse{}, errors.Errorf("failed to run unknown test %v", req.Test)
	}
	cfg := getTestConfig(outDir)
	cfg.CameraFacing = map[cameraboxpb.Facing]string{
		cameraboxpb.Facing_FACING_BACK:  "back",
		cameraboxpb.Facing_FACING_FRONT: "front",
	}[req.Facing]

	retErr := hal3.RunTest(ctx, cfg)
	out, err := testexec.CommandContext(ctx, "tar", "czOC", outDir, ".").Output(testexec.DumpLogOnError)
	if err != nil {
		if retErr == nil {
			retErr = errors.Wrap(err, "failed to compress log directory")
		} else {
			testing.ContextLog(ctx, "Failed to compress log directory: ", err)
		}
		return &cameraboxpb.RunTestResponse{}, retErr
	}

	return &cameraboxpb.RunTestResponse{OutDir: out}, retErr
}
