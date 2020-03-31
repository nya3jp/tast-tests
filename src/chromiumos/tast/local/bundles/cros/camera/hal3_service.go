// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"io/ioutil"
	"os"

	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/hal3"
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

func (c *HAL3Service) RunTest(ctx context.Context, req *cameraboxpb.RunTestRequest) (_ *cameraboxpb.RunTestResponse, retErr error) {
	outDir, err := createOutDir(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create remote output directory")
	}
	defer func() {
		if retErr != nil && len(outDir) > 0 {
			if err := os.Remove(outDir); err != nil {
				testing.ContextLog(ctx, "Failed to remove temp output directory in error cleanup: ", err)
			}
		}
	}()

	getTestConfig, ok := getTestConfigMap[req.Test]
	if !ok {
		return nil, errors.Errorf("failed to run unknown test %v", req.Test)
	}
	cfg := getTestConfig(outDir)
	switch req.Facing {
	case cameraboxpb.Facing_FACING_BACK:
		cfg.CameraFacing = "back"
	case cameraboxpb.Facing_FACING_FRONT:
		cfg.CameraFacing = "front"
	default:
		return nil, errors.Errorf("unsupported facing: %v", req.Facing.String())
	}
	cfg.CameraHALs = []string{}

	result := cameraboxpb.RunTestResponse{OutPath: outDir}
	if testErr := hal3.RunTest(ctx, cfg); testErr == nil {
		result.Result = cameraboxpb.TestResult_TEST_RESULT_PASSED
	} else {
		result.Result = cameraboxpb.TestResult_TEST_RESULT_FAILED
		result.Error = testErr.Error()
	}
	return &result, nil
}

func createOutDir(ctx context.Context) (_ string, retErr error) {
	outDir, err := ioutil.TempDir("", "hal3_outdir_")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp output directory")
	}
	defer func() {
		if retErr != nil {
			if err := os.Remove(outDir); err != nil {
				testing.ContextLog(ctx, "Failed to remove temp directory in error cleanup: ", err)
			}
		}
	}()

	uid, err := hal3.ArcCameraUID()
	if err != nil {
		return "", err
	}
	if err := os.Chown(outDir, uid, -1); err != nil {
		return "", errors.Wrapf(err, "failed to chown temp output directory %v to arc-camera", outDir)
	}
	return outDir, nil
}
