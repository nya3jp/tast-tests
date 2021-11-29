// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"os"
	"path"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	commoncros "chromiumos/tast/common/cros"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	svcdef "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	var screenRecorderService ScreenRecorderService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			screenRecorderService = ScreenRecorderService{s: s, sharedObject: commoncros.SharedObjectsForServiceSingleton}
			svcdef.RegisterScreenRecorderServiceServer(srv, &screenRecorderService)
		},
		GuaranteeCompatibility: true,
	})
}

// ScreenRecorderService implements tast.cros.ui.ScreenRecorderService
type ScreenRecorderService struct {
	s              *testing.ServiceState
	sharedObject   *commoncros.SharedObjectsForService
	screenRecorder *uiauto.ScreenRecorder
}

func (svc *ScreenRecorderService) Start(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if svc.screenRecorder != nil {
		return nil, errors.New("Cannot start again when recording is in progress")
	}

	cr := svc.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("Chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Failed to create test API connection")
	}

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil || screenRecorder == nil {
		return nil, errors.Wrap(err, "Failed to create ScreenRecorder: ")
	}
	svc.screenRecorder = screenRecorder
	svc.screenRecorder.Start(ctx, tconn)

	return &empty.Empty{}, nil
}

func (svc *ScreenRecorderService) StopSaveRelease(ctx context.Context, req *svcdef.StopSaveReleaseRequest) (*empty.Empty, error) {
	if svc.screenRecorder == nil {
		return nil, errors.New("Failed to stop when no recording is progress")
	}
	dir := path.Dir(req.FileName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create a dir at "+dir)
	}
	uiauto.ScreenRecorderStopSaveRelease(ctx, svc.screenRecorder, req.FileName)
	svc.screenRecorder = nil
	return &empty.Empty{}, nil
}

//TODO: Decide if we want to move to help,
// or be an instance method
// func getUIAutoContext1(ctx context.Context, svc *ScreenRecorderService) (*uiauto.Context, error) {
// 	cr := svc.sharedObject.Chrome
// 	if cr == nil {
// 		return nil, errors.New("Chrome is not instantiated")
// 	}
// 	tconn, err := cr.TestAPIConn(ctx)
// 	if err != nil {
// 		return nil, errors.Wrap(err, "Failed to create test API connection")
// 	}
// 	ui := uiauto.New(tconn)
// 	return ui, nil
// }
