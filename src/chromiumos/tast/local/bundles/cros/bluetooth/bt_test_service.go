// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"

	"google.golang.org/grpc"
	"google.golang.org/protobuf/types/known/emptypb"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bluetooth"
	"chromiumos/tast/local/chrome"
	pb "chromiumos/tast/services/cros/bluetooth"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterBTTestServiceServer(srv, &BTTestService{s: s})
		},
	})
}

// BTTestService implements tast.cros.bluetooth.BTTestService.
type BTTestService struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

// ChromeNew logs into chrome. ChromeClose must be called later.
func (bts *BTTestService) ChromeNew(ctx context.Context, request *pb.ChromeNewRequest) (*emptypb.Empty, error) {
	if bts.cr != nil {
		return nil, errors.New("chrome already available")
	}
	var chromeOpts []chrome.Option
	if request.BluetoothRevampEnabled {
		chromeOpts = []chrome.Option{chrome.EnableFeatures("BluetoothRevamp")}
	} else {
		chromeOpts = []chrome.Option{chrome.DisableFeatures("BluetoothRevamp")}
	}
	cr, err := chrome.New(ctx, chromeOpts...)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create new Chrome")
	}
	bts.cr = cr
	return &emptypb.Empty{}, nil
}

// ChromeClose cleans up resources from ChromeNew.
func (bts *BTTestService) ChromeClose(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if bts.cr == nil {
		return nil, errors.New("no chrome to close, call ChromeNew first")
	}
	if err := bts.cr.Close(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to close chrome")
	}
	return &emptypb.Empty{}, nil
}

// EnableBluetoothAdapter powers on the bluetooth adapter.
func (bts *BTTestService) EnableBluetoothAdapter(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if err := bluetooth.Enable(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to enable bluetooth adapter")
	}
	return &emptypb.Empty{}, nil
}

// DisableBluetoothAdapter powers off the bluetooth adapter.
func (bts *BTTestService) DisableBluetoothAdapter(ctx context.Context, empty *emptypb.Empty) (*emptypb.Empty, error) {
	if err := bluetooth.Disable(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to disable bluetooth adapter")
	}
	return &emptypb.Empty{}, nil
}
