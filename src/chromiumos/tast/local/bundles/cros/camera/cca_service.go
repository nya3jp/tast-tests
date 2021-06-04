// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"os"

	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/syslog"
	cameraboxpb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			cameraboxpb.RegisterCCAServiceServer(srv, &CCAService{s: s})
		},
	})
}

type CCAService struct {
	s *testing.ServiceState
}

type testFunc = func(ctx context.Context, scriptPaths []string, outDir string, facing cca.Facing) error

var getTestFuncMap = map[cameraboxpb.CCATest]testFunc{
	cameraboxpb.CCATest_DOCUMENT_SCANNING: cca.RunPreviewDocumentCornersDetection,
}

var facingMap = map[cameraboxpb.Facing]cca.Facing{
	cameraboxpb.Facing_FACING_UNSET: cca.FacingFront,
	cameraboxpb.Facing_FACING_BACK:  cca.FacingBack,
	cameraboxpb.Facing_FACING_FRONT: cca.FacingFront,
}

func tempFilePathForScriptContent(ctx context.Context, script string) (*os.File, error) {
	tempFile, err := ioutil.TempFile("", "Script_*")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create temp file for script")
	}
	_, err = tempFile.Write([]byte(script))
	if err != nil {
		return nil, errors.Wrap(err, "failed to write script into temp file")
	}
	if err := tempFile.Close(); err != nil {
		return nil, errors.Wrap(err, "failed to close temp file")
	}
	return tempFile, nil
}

func (c *CCAService) RunTest(ctx context.Context, req *cameraboxpb.CCATestRequest) (_ *cameraboxpb.CCATestResponse, retErr error) {
	testFunc, ok := getTestFuncMap[req.Test]
	if !ok {
		return nil, errors.Errorf("failed to run unknown test %v", req.Test)
	}

	endLogFn, err := syslog.CollectSyslog()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start collecting syslog")
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get remote output directory")
	}

	var tmpScriptPaths []string
	for _, scriptContent := range req.ScriptContents {
		tempFile, err := tempFilePathForScriptContent(ctx, scriptContent)
		if err != nil {
			return nil, errors.Wrap(err, "failed to put script contents into temp file")
		}
		tmpScriptPaths = append(tmpScriptPaths, tempFile.Name())
	}
	defer func() {
		for _, path := range tmpScriptPaths {
			os.Remove(path)
		}
	}()

	result := cameraboxpb.CCATestResponse{}
	if testErr := testFunc(ctx, tmpScriptPaths, outDir, facingMap[req.Facing]); testErr == nil {
		result.Result = cameraboxpb.TestResult_TEST_RESULT_PASSED
	} else {
		result.Result = cameraboxpb.TestResult_TEST_RESULT_FAILED
		result.Error = testErr.Error()
	}

	if err := endLogFn(ctx, outDir); err != nil {
		return nil, errors.Wrap(err, "failed to finish collecting syslog")
	}

	return &result, nil
}
