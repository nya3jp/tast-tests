// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"strings"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/camera/cca"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/mtbf/camera"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			camera.RegisterCameraServiceServer(srv, &Service{chrome.SvcLoginReusePre{S: s}})
		},
	})
}

// Service implements tast.mtbf.svc.Camera.
type Service struct {
	// embedded structure to get the implementation of LoginReusePre
	chrome.SvcLoginReusePre
}

// New open cca
func (s *Service) New(ctx context.Context, outDir string) (*cca.App, error) {
	// prepare the Chrome instance just in case
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return nil, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}
	dataPath := []string{"/home/chronos/user/Downloads/data/cca_ui.js"}
	app, err := cca.New(ctx, s.CR, dataPath, outDir)
	if err != nil {
		if strings.Contains(err.Error(), "Chrome probably crashed") {
			return nil, mtbferrors.New(mtbferrors.CmrChromeCrashed, err)
		}
		return nil, mtbferrors.New(mtbferrors.CmrOpenCCA, err)
	}

	if err := app.WaitForVideoActive(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.CmrInact, err)
	}

	return app, nil
}

// GetConnection get cca connection
func (s *Service) GetConnection(ctx context.Context, outDir string) (*cca.App, error) {
	// prepare the Chrome instance just in case
	if err := s.PrePrepare(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return nil, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}
	dataPath := []string{"/home/chronos/user/Downloads/data/cca_ui.js"}

	app, err := cca.GetConnection(ctx, s.CR, dataPath, outDir)
	if err != nil {
		if strings.Contains(err.Error(), "Chrome probably crashed") {
			return nil, mtbferrors.New(mtbferrors.CmrChromeCrashed, err)
		}
		return nil, mtbferrors.New(mtbferrors.CmrOpenCCA, err)
	}
	if err := app.WaitForVideoActive(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.CmrInact, err)
	}
	return app, nil
}

// SwitchToPortraitMode switch to portrait mode.
func (s *Service) SwitchToPortraitMode(ctx context.Context, req *camera.SwitchToPortraitModeRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "CameraService - SwitchToPortraitMode called")
	app, mtbferr := s.New(ctx, req.OutDir)
	if mtbferr != nil {
		return &empty.Empty{}, mtbferr
	}

	// Switch to portrait mode
	testing.ContextLog(ctx, "Supported portrait mode")
	const portraitModeSelector = "Tast.isVisible('#modes-group > .mode-item:last-child')"
	if err := app.CheckElementExist(ctx, portraitModeSelector, true); err == nil {
		if err := app.SwitchMode(ctx, cca.Portrait); err != nil {
			return &empty.Empty{}, mtbferrors.New(mtbferrors.CmrPortrait, err)
		}
	}

	return &empty.Empty{}, nil
}

// SwitchCamera switch camera.
func (s *Service) SwitchCamera(ctx context.Context, req *camera.SwitchCameraRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "CameraService - SwitchCamera called")
	app, mtbferr := s.GetConnection(ctx, req.OutDir)
	if mtbferr != nil {
		return &empty.Empty{}, mtbferr
	}

	if err := app.SwitchCamera(ctx); err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.CmrSwitch, err)
	}

	if err := app.WaitForVideoActive(ctx); err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.CmrInact, err)
	}

	return &empty.Empty{}, nil
}

// GetNumOfCameras get num of cameras.
func (s *Service) GetNumOfCameras(ctx context.Context, req *camera.GetNumOfCamerasRequest) (*camera.GetNumOfCamerasResponse, error) {
	testing.ContextLog(ctx, "CameraService - GetNumOfCameras called")
	app, mtbferr := s.GetConnection(ctx, req.OutDir)
	if mtbferr != nil {
		return nil, mtbferr
	}

	numCameras, err := app.GetNumOfCameras(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.CmrNumber, err)
	}

	return &camera.GetNumOfCamerasResponse{Num: int64(numCameras)}, nil
}

// GetModeState get camera mode state.
func (s *Service) GetModeState(ctx context.Context, req *camera.GetModeStateRequest) (*camera.GetModeStateResponse, error) {
	testing.ContextLog(ctx, "CameraService - GetModeState called")
	app, mtbferr := s.GetConnection(ctx, req.OutDir)
	if mtbferr != nil {
		return nil, mtbferr
	}
	// Check mode selector fallback to photo mode.
	active, err := app.GetState(ctx, req.Mode)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.CmrAppState, err)
	} else if !active {
		return nil, mtbferrors.New(mtbferrors.CmrFallBack, nil)
	}

	return &camera.GetModeStateResponse{Active: active}, nil
}

// CheckElementExist check camera element exist.
func (s *Service) CheckElementExist(ctx context.Context, req *camera.CheckElementExistRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "CameraService - CheckElementExist called")
	app, mtbferr := s.GetConnection(ctx, req.OutDir)
	if mtbferr != nil {
		return &empty.Empty{}, mtbferr
	}
	// Check the portrait mode icon should disappear
	const portraitModeSelector = "Tast.isVisible('#modes-group > .mode-item:last-child')"
	if err := app.CheckElementExist(ctx, portraitModeSelector, false); err != nil {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.CmrPortraitBtn, err)
	}

	return &empty.Empty{}, nil
}

// CloseCamera close camera.
func (s *Service) CloseCamera(ctx context.Context, req *camera.CloseCameraRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "CameraService - CloseCamera called")
	app, mtbferr := s.GetConnection(ctx, req.OutDir)
	if mtbferr != nil {
		return &empty.Empty{}, mtbferr
	}
	defer app.Close(ctx)
	return &empty.Empty{}, nil
}
