// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package oobe

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/common"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/oobe"
	pb "chromiumos/tast/services/cros/oobe"
	"chromiumos/tast/testing"
)

func init() {
	var hidScreenService HidScreenService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			hidScreenService = HidScreenService{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterHidScreenServiceServer(srv, &hidScreenService)
		},
		GuaranteeCompatibility: true,
	})
}

// HidScreenService implements tast.cros.oobe.HidScreenService.
type HidScreenService struct {
	// sharedObject is used to access current chrome instance.
	sharedObject *common.SharedObjectsForService
}

func (svc *HidScreenService) ConnectAndVerifyMouse(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr := svc.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome is not instantiated")
	}

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create OOBE connection")
	}
	defer oobeConn.Close()

	if err := oobe.IsHidMouseDetected(ctx, oobeConn); err == nil {
		return &empty.Empty{}, errors.Wrap(err, "expected no mouse device to be detected")
	}

	if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err == nil {
		return &empty.Empty{}, errors.Wrap(err, "expected continue button to be disabled")
	}

	mouseDvc, err := input.Mouse(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get mouse handle")
	}
	defer mouseDvc.Close()

	if err := oobe.IsHidMouseDetected(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "expected mouse device to be detected")
	}

	if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "expected continue button to be enabled")
	}

	return &empty.Empty{}, nil
}

func (svc *HidScreenService) DisconnectAndVerifyMouse(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr := svc.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome is not instantiated")
	}

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create OOBE connection")
	}
	defer oobeConn.Close()

	mouseDvc, err := input.Mouse(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get mouse handle")
	}
	defer mouseDvc.Close()

	if err := oobe.IsHidMouseDetected(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "expected mouse device to be detected")
	}

	if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "expected continue button to be enabled")
	}

	mouseDvc.Close()

	if err := oobe.IsHidMouseDetected(ctx, oobeConn); err == nil {
		return &empty.Empty{}, errors.Wrap(err, "expected no mouse device to be detected")
	}

	if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err == nil {
		return &empty.Empty{}, errors.Wrap(err, "expected continue button to be disabled")
	}

	return &empty.Empty{}, nil
}

func (svc *HidScreenService) ConnectAndVerifyKeyboard(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr := svc.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome is not instantiated")
	}

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create OOBE connection")
	}
	defer oobeConn.Close()

	if err := oobe.IsHidKeyboardDetected(ctx, oobeConn); err == nil {
		return &empty.Empty{}, errors.Wrap(err, "expected no keyboard device to be detected")
	}

	if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err == nil {
		return &empty.Empty{}, errors.Wrap(err, "expected continue button to be disabled")
	}

	keyboardDvc, err := input.Keyboard(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get keyboard handle")
	}
	defer keyboardDvc.Close()

	if err := oobe.IsHidKeyboardDetected(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "expected keyboard device to be detected")
	}

	if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "expected continue button to be enabled")
	}

	return &empty.Empty{}, nil
}

func (svc *HidScreenService) DisconnectAndVerifyKeyboard(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr := svc.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome is not instantiated")
	}

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create OOBE connection")
	}
	defer oobeConn.Close()

	keyboardDvc, err := input.Keyboard(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to get keyboard handle")
	}
	defer keyboardDvc.Close()

	if err := oobe.IsHidKeyboardDetected(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "expected keyboard device to be detected")
	}

	if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "expected continue button to be enabled")
	}

	keyboardDvc.Close()

	if err := oobe.IsHidKeyboardDetected(ctx, oobeConn); err == nil {
		return &empty.Empty{}, errors.Wrap(err, "expected no keyboard device to be detected")
	}

	if err := oobe.IsHidDetectionContinueButtonEnabled(ctx, oobeConn); err == nil {
		return &empty.Empty{}, errors.Wrap(err, "expected continue button to be disabled")
	}

	return &empty.Empty{}, nil
}
