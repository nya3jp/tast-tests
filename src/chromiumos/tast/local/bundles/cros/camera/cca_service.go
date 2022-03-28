// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"os"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/camera/cca"
	"chromiumos/tast/local/camera/testutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/camera"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			camera.RegisterCCAServiceServer(srv, &CCAService{s: s})
		},
	})
}

// CCAService implements tast.cros.camera.CCAService.
type CCAService struct {
	s              *testing.ServiceState
	cr             *chrome.Chrome
	app            *cca.App
	tb             *testutil.TestBridge
	tmpScriptPaths *[]string
}

var modeMap = map[camera.CameraMode]cca.Mode{
	camera.CameraMode_PHOTO: cca.Photo,
	camera.CameraMode_VIDEO: cca.Video,
}

var cameraFacingMap = map[camera.Facing]cca.Facing{
	camera.Facing_FACING_BACK:  cca.FacingBack,
	camera.Facing_FACING_FRONT: cca.FacingFront,
}

// tempFilePathForScript creates a temp file and writes the camera JS script in that file.
func tempFilePathForScript(ctx context.Context, script []byte) (string, error) {
	tempFile, err := ioutil.TempFile("", "Script_*")
	if err != nil {
		return "", errors.Wrap(err, "failed to create temp file for script")
	}
	defer tempFile.Close()

	_, err = tempFile.Write(script)
	if err != nil {
		return "", errors.Wrap(err, "failed to write script into temp file")
	}
	return tempFile.Name(), nil
}

// NewChrome logs into a Chrome session as a user. CloseChrome must be called later
// to clean up the associated resources.
func (c *CCAService) NewChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	c.cr = cr
	return &empty.Empty{}, nil
}

// CloseChrome releases the resources obtained by NewChrome.
func (c *CCAService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := c.cr.Close(ctx)
	c.cr = nil
	return &empty.Empty{}, err
}

// ReuseChrome passes an Option to New to make Chrome reuse the existing login session.
func (c *CCAService) ReuseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx, chrome.TryReuseSession())
	if err != nil {
		return nil, err
	}
	c.cr = cr
	return &empty.Empty{}, nil
}

// OpenCamera launches the specific camera with photo or video mode.
func (c *CCAService) OpenCamera(ctx context.Context, req *camera.CameraTestRequest) (*camera.CameraTestResponse, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get remote output directory")
	}
	var tmpScriptPaths []string
	for _, scriptContent := range req.ScriptContents {
		tempFilePath, err := tempFilePathForScript(ctx, scriptContent)
		if err != nil {
			return nil, errors.Wrap(err, "failed to put script contents into temp file")
		}
		tmpScriptPaths = append(tmpScriptPaths, tempFilePath)
	}
	c.tmpScriptPaths = &tmpScriptPaths

	tb, err := testutil.NewTestBridge(ctx, c.cr, testutil.UseRealCamera)
	if err != nil {
		return nil, errors.Wrap(err, "failed to construct test bridge")
	}
	c.tb = tb

	if err := cca.ClearSavedDir(ctx, c.cr); err != nil {
		return nil, errors.Wrap(err, "failed to clear saved directory")
	}

	app, err := cca.New(ctx, c.cr, tmpScriptPaths, outDir, tb)
	if err != nil {
		return nil, errors.Wrap(err, "failed to open CCA")
	}
	c.app = app

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to get number of cameras")
	}
	testing.ContextLogf(ctx, "No. of cameras: %d", numCameras)

	wantFacing := cameraFacingMap[req.Facing]
	wantMode := modeMap[req.Mode]
	// Check whether back camera is available or not.
	if wantFacing == cca.FacingBack && numCameras == 1 {
		return nil, errors.Errorf("failed to test as %v camera doesn't exist", wantFacing)
	}

	// Check whether correct camera is switched.
	checkFacing := func() (bool, error) {
		facing, err := app.GetFacing(ctx)
		if err != nil {
			return false, errors.Wrap(err, "failed to get facing")
		}
		return facing == wantFacing, nil
	}

	// Verify the camera facing for user facing and env facing params.
	facingStatus, err := checkFacing()
	if err != nil {
		return nil, errors.Wrap(err, "failed to get the camera facing")
	}
	if !facingStatus {
		if err := app.SwitchCamera(ctx); err != nil {
			return nil, errors.Wrap(err, "failed to switch camera")
		}
		facingStatus, err := checkFacing()
		if err != nil {
			return nil, errors.Wrap(err, "failed to get the camera facing")
		}
		if !facingStatus {
			return nil, errors.Errorf("failed to get default camera facing as %v", wantFacing)
		}
	}

	if err := app.SwitchMode(ctx, wantMode); err != nil {
		return nil, errors.Wrapf(err, "failed to switch to %v viewfinder", wantMode)
	}

	result := camera.CameraTestResponse{}
	result.Result = camera.TestResult_TEST_RESULT_PASSED
	return &result, nil
}

// CloseCamera releases the resources obtained by OpenCamera.
func (c *CCAService) CloseCamera(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	err := c.app.Close(ctx)
	c.tb.TearDown(ctx)
	for _, path := range *c.tmpScriptPaths {
		os.Remove(path)
	}
	return &empty.Empty{}, err
}

// TakePicture captures a photo using the camera.
func (c *CCAService) TakePicture(ctx context.Context, req *empty.Empty) (*camera.CameraTestResponse, error) {
	result := camera.CameraTestResponse{}
	if _, testErr := c.app.TakeSinglePhoto(ctx, cca.TimerOff); testErr == nil {
		result.Result = camera.TestResult_TEST_RESULT_PASSED
	} else {
		result.Result = camera.TestResult_TEST_RESULT_FAILED
		result.Error = testErr.Error()
	}
	return &result, nil
}

// RecordVideo records a video using the camera.
func (c *CCAService) RecordVideo(ctx context.Context, req *empty.Empty) (*camera.CameraTestResponse, error) {
	result := camera.CameraTestResponse{}
	if _, testErr := c.app.RecordVideo(ctx, cca.TimerOff, 10*time.Second); testErr == nil {
		result.Result = camera.TestResult_TEST_RESULT_PASSED
	} else {
		result.Result = camera.TestResult_TEST_RESULT_FAILED
		result.Error = testErr.Error()
	}
	return &result, nil
}

// CheckCameraExists checks if the camera instance exists.
func (c *CCAService) CheckCameraExists(ctx context.Context, req *empty.Empty) (*camera.CameraTestResponse, error) {
	result := camera.CameraTestResponse{}
	appExist, err := cca.InstanceExists(ctx, c.cr)
	if err == nil {
		if appExist {
			result.Result = camera.TestResult_TEST_RESULT_PASSED
		} else {
			result.Result = camera.TestResult_TEST_RESULT_FAILED
		}
	} else {
		result.Result = camera.TestResult_TEST_RESULT_FAILED
		result.Error = err.Error()
	}
	return &result, nil
}
