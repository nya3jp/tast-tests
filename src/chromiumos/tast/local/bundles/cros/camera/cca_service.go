// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
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

func tempFilePathForScriptContent(ctx context.Context, script string) (string, error) {
	tempFile, err := ioutil.TempFile("", "Script_*")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file for script")
	}
	defer tempFile.Close()

	_, err = tempFile.Write([]byte(script))
	if err != nil {
		return "", errors.Wrap(err, "failed to write script into temp file")
	}
	return tempFile.Name(), nil
}

func (c *CCAService) RunTest(ctx context.Context, req *cameraboxpb.CCATestRequest) (_ *cameraboxpb.CCATestResponse, retErr error) {
	testFunc, ok := getTestFuncMap[req.Test]
	if !ok {
		return nil, errors.Errorf("failed to run unknown test %v", req.Test)
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get remote output directory")
	}

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	endLogFn, err := syslog.CollectSyslog()
	if err != nil {
		return nil, errors.Wrap(err, "failed to start collecting syslog")
	}
	defer func(ctx context.Context) {
		endLogFn(ctx, outDir)
	}(cleanupCtx)

	var tmpScriptPaths []string
	for _, scriptContent := range req.ScriptContents {
		tempFilePath, err := tempFilePathForScriptContent(ctx, scriptContent)
		if err != nil {
			return nil, errors.Wrap(err, "failed to put script contents into temp file")
		}
		tmpScriptPaths = append(tmpScriptPaths, tempFilePath)
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

	return &result, nil
}
