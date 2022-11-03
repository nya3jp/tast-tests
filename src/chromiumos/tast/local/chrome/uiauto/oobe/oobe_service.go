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
	pb "chromiumos/tast/services/cros/oobe/hiddetection"
	"chromiumos/tast/testing"
)

func init() {
	var OobeService Service
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			oobeService = Service{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterHidDetectionServiceServer(srv, &hidDetectionService)
		},
	})
}

// Service implements tast.cros.chrome.uiauto.hiddetection.HidDetectionService
type Service struct {
	sharedObject *common.SharedObjectsForService
}

// HidPreconnectedTouchscreenOnly will navigate to the oobe hid detection page.
func (s *Service) HidPreconnectedTouchscreenOnly(ctx context.Context, e *empty.Empty) *empty.Empty {
	cr := s.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome has not been started")
	}

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create test API connection")
	}

	// oobeConn, err := cr.WaitForOOBEConnection(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to create OOBE connection: ", err)
	// }
	// defer oobeConn.Close()

	// if err := oobe.IsHidDetectionScreenVisible(ctx, oobeConn); err != nil {
	// 	s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	// }

	// if err := oobe.IsHidDetectionTouchscreenDetected(ctx, oobeConn); err != nil {
	// 	s.Fatal("Failed to find the text indicating that a pointer is connected: ", err)
	// }

	// if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
	// 	s.Fatal("Failed to detect an enabled continue button: ", err)
	// }

	// tconn, err := cr.SigninProfileTestAPIConn(ctx)
	// if err != nil {
	// 	s.Fatal("Failed to create the signin profile test API connection: ", err)
	// }

	// if err := oobe.IsHidDetectionKeyboardNotDetected(ctx, oobeConn, tconn); err != nil {
	// 	s.Fatal("Failed to detect that no keyboard was detected: ", err)
	// }

	return &empty.Empty{}
}
