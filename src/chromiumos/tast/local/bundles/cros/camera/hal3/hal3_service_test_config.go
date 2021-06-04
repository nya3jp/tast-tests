// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package hal3

import (
	cameraboxpb "chromiumos/tast/services/cros/camerabox"
)

// ServiceTestConfigGenerator generates |TestConfig| from test request for HAL3Service.
type ServiceTestConfigGenerator interface {
	TestConfig(req *cameraboxpb.RunTestRequest) TestConfig
}

// defaultServiceCfgGenerator implements |ServiceTestConfigGenerator| and
// generates |TestConfig| directly from |getTestConfig|.
type defaultServiceCfgGenerator struct {
	getTestConfig func() TestConfig
}

// TestConfig gets test config for running hal3 test.
func (gen defaultServiceCfgGenerator) TestConfig(_ *cameraboxpb.RunTestRequest) TestConfig {
	return gen.getTestConfig()
}

// FaceDetectionTestConfig returns test config for running Camera3FaceDetection test.
func FaceDetectionTestConfig() TestConfig {
	return TestConfig{
		GtestFilter:            "*Camera3FaceDetection*",
		ConnectToCameraService: true,
	}
}

// faceDetectionServiceCfgGenerator implements |ServiceTestConfigGenerator| and
// generates |TestConfig| directly from |getTestConfig|.
type faceDetectionServiceCfgGenerator struct {
	getTestConfig func() TestConfig
}

// TestConfig gets test config for running hal3test for face detection.
func (gen faceDetectionServiceCfgGenerator) TestConfig(req *cameraboxpb.RunTestRequest) TestConfig {
	cfg := gen.getTestConfig()
	cfg.ExpectedNumFaces = req.ExtendedParams
	return cfg
}

// ServiceTestConfigGenerators maps from test type to test config generator for HAL3Service.
var ServiceTestConfigGenerators = map[cameraboxpb.HAL3CameraTest]ServiceTestConfigGenerator{
	cameraboxpb.HAL3CameraTest_DEVICE:         defaultServiceCfgGenerator{DeviceTestConfig},
	cameraboxpb.HAL3CameraTest_FRAME:          defaultServiceCfgGenerator{FrameTestConfig},
	cameraboxpb.HAL3CameraTest_JDA:            defaultServiceCfgGenerator{JDATestConfig},
	cameraboxpb.HAL3CameraTest_JEA:            defaultServiceCfgGenerator{JEATestConfig},
	cameraboxpb.HAL3CameraTest_MODULE:         defaultServiceCfgGenerator{ModuleTestConfig},
	cameraboxpb.HAL3CameraTest_PERF:           defaultServiceCfgGenerator{PerfTestConfig},
	cameraboxpb.HAL3CameraTest_PREVIEW:        defaultServiceCfgGenerator{PreviewTestConfig},
	cameraboxpb.HAL3CameraTest_RECORDING:      defaultServiceCfgGenerator{RecordingTestConfig},
	cameraboxpb.HAL3CameraTest_STILL_CAPTURE:  defaultServiceCfgGenerator{StillCaptureTestConfig},
	cameraboxpb.HAL3CameraTest_STREAM:         defaultServiceCfgGenerator{StreamTestConfig},
	cameraboxpb.HAL3CameraTest_FACE_DETECTION: faceDetectionServiceCfgGenerator{FaceDetectionTestConfig},
}
