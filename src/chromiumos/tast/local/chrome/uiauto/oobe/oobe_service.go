// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package oobe is for controlling the OOBE directly from the UI.
package oobe

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/common"
	oobeHelper "chromiumos/tast/local/oobe"
	pb "chromiumos/tast/services/cros/chrome/uiauto/oobe"
	"chromiumos/tast/testing"
)

func init() {
	var oobeService Service
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			oobeService = Service{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterOobeServiceServer(srv, &oobeService)
		},
	})
}

// Service implements tast.cros.chrome.uiauto.oobe.oobeService
type Service struct {
	sharedObject *common.SharedObjectsForService
}

// HidPreconnectedTouchscreenOnly will navigate to the oobe hid detection page.
func (s *Service) HidPreconnectedTouchscreenOnly(ctx context.Context, e *empty.Empty) *empty.Empty {
	cr := s.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("chrome has not been started")
	}

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		return &empty.Empty{}, errors.New("failed to create OOBE connection")
	}
	defer oobeConn.Close()

	if err := oobeHelper.IsHidDetectionScreenVisible(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.New("failed to wait for the welcome screen to be visible")
	}

	if err := oobeHelper.IsHidDetectionTouchscreenDetected(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.New("failed to find the text indicating that a pointer is connected")
	}

	if err := oobeHelper.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.New("failed to detect an enabled continue button")
	}

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, errors.New("failed to create the signin profile test API connection")
	}

	if err := oobeHelper.IsHidDetectionKeyboardNotDetected(ctx, oobeConn, tconn); err != nil {
		return &empty.Empty{}, errors.New("failed to detect that no keyboard was detected")
	}

	return &empty.Empty{}
}
