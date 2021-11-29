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

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto"
	common "chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	var screenRecorderService ScreenRecorderService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			screenRecorderService = ScreenRecorderService{s: s, sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterScreenRecorderServiceServer(srv, &screenRecorderService)
		},
		GuaranteeCompatibility: true,
	})
}

// ScreenRecorderService implements tast.cros.ui.ScreenRecorderService
type ScreenRecorderService struct {
	s              *testing.ServiceState
	sharedObject   *common.SharedObjectsForService
	screenRecorder *uiauto.ScreenRecorder
}

func (svc *ScreenRecorderService) Start(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if svc.screenRecorder != nil {
		return nil, errors.New("cannot start again when recording is in progress")
	}

	cr := svc.sharedObject.Chrome
	if cr == nil {
		return nil, errors.New("chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test API connection")
	}

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil || screenRecorder == nil {
		return nil, errors.Wrap(err, "failed to create ScreenRecorder")
	}
	svc.screenRecorder = screenRecorder
	svc.screenRecorder.Start(ctx, tconn)

	return &empty.Empty{}, nil
}

func (svc *ScreenRecorderService) StopSaveRelease(ctx context.Context, req *pb.StopSaveReleaseRequest) (*empty.Empty, error) {
	if svc.screenRecorder == nil {
		return nil, errors.New("failed to stop when no recording is progress")
	}
	dir := path.Dir(req.FileName)
	if err := os.MkdirAll(dir, 0755); err != nil {
		return nil, errors.Wrap(err, "failed to create a dir at "+dir)
	}
	uiauto.ScreenRecorderStopSaveRelease(ctx, svc.screenRecorder, req.FileName)
	svc.screenRecorder = nil
	return &empty.Empty{}, nil
}
