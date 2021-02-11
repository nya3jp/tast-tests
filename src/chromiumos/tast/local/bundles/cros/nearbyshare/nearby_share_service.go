// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package nearbyshare

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	localnearby "chromiumos/tast/local/chrome/nearbyshare"
	"chromiumos/tast/local/chrome/nearbyshare/nearbysetup"
	"chromiumos/tast/services/cros/nearbyshare"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			nearbyshare.RegisterNearbyShareServiceServer(srv, &NearbyService{s: s})
		},
	})
}

// NearbyService implements tast.cros.nearbyshare.NearbyShareService.
type NearbyService struct {
	s *testing.ServiceState

	cr         *chrome.Chrome
	tconn      *chrome.TestConn
	deviceName string
}

func (n *NearbyService) NewChromeLogin(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(
		ctx,
		chrome.EnableFeatures("IntentHandlingSharing", "NearbySharing", "Sharesheet"),
	)
	if err != nil {
		testing.ContextLog(ctx, "Failed to start Chrome")
		return nil, err
	}
	n.cr = cr
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		testing.ContextLog(ctx, "Failed to get a connection to the Test Extension")
		return nil, err
	}
	n.tconn = tconn
	return &empty.Empty{}, nil
}

func (n *NearbyService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := n.cr.Close(ctx)
	n.cr = nil
	return &empty.Empty{}, err
}

func (n *NearbyService) CrOSSetup(ctx context.Context, req *nearbyshare.CrOSSetupRequest) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	n.deviceName = req.DeviceName
	return &empty.Empty{}, nearbysetup.CrOSSetup(ctx, n.tconn, n.cr, nearbysetup.DataUsage(req.DataUsage), nearbysetup.Visibility(req.Visibility), req.DeviceName)
}

func (n *NearbyService) StartHighVisibilityMode(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if n.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	return &empty.Empty{}, localnearby.StartHighVisibilityMode(ctx, n.tconn, n.deviceName)
}
