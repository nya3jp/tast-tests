// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"

	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/camera/hal3"
	"chromiumos/tast/local/syslog"
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

func (c *HAL3Service) RunTest(ctx context.Context, req *cameraboxpb.RunTestRequest) (_ *cameraboxpb.RunTestResponse, retErr error) {
	generator, ok := hal3.ServiceTestConfigGenerators[req.Test]
	if !ok {
		return nil, errors.Errorf("failed to run unknown test %v", req.Test)
	}
	cfg := generator.TestConfig(req)
	switch req.Facing {
	case cameraboxpb.Facing_FACING_BACK:
		cfg.CameraFacing = "back"
	case cameraboxpb.Facing_FACING_FRONT:
		cfg.CameraFacing = "front"
	default:
		return nil, errors.Errorf("unsupported facing: %v", req.Facing.String())
	}
	cfg.CameraHALs = []string{}

	endLogFn, err := syslog.CollectSyslog()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start collecting syslog")
	}

	result := cameraboxpb.RunTestResponse{}
	if testErr := hal3.RunTest(ctx, cfg); testErr == nil {
		result.Result = cameraboxpb.TestResult_TEST_RESULT_PASSED
	} else {
		result.Result = cameraboxpb.TestResult_TEST_RESULT_FAILED
		result.Error = testErr.Error()
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get remote output directory")
	}
	if err := endLogFn(ctx, outDir); err != nil {
		return nil, errors.Wrap(err, "failed to finish collecting syslog")
	}

	return &result, nil
}
