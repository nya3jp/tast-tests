// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package wifi

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/wifi"
	ws "chromiumos/tast/services/mtbf/wifi"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			ws.RegisterWifiServiceServer(srv, &Service{chrome.SvcLoginReusePre{S: s}})
		},
	})
}

// Service implements tast.mtbf.wifi.WifiService.
type Service struct {
	chrome.SvcLoginReusePre
}

// Disable disables Wi-Fi.
func (s *Service) Disable(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	testing.ContextLog(ctx, "WifiService.Disable called")

	err := s.PrePrepare(ctx) // prepare the Chrome instance just in case
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return nil, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	wifiConn, mtbferr := wifi.NewConn(ctx, s.CR, false, "", "", "", "")
	if mtbferr != nil {
		return nil, mtbferr
	}
	defer wifiConn.Close(true)

	wifiConn.DisableWifi()

	testing.ContextLog(ctx, "WifiService.Disable succeeded")

	return &empty.Empty{}, nil
}

// Verify verifies whether Wi-Fi service is enable or not.
func (s *Service) Verify(ctx context.Context, req *ws.VerifyRequest) (*ws.VerifyResponse, error) {
	testing.ContextLogf(ctx, "WifiService.Verify called, enable=%v", req.Enable)

	err := s.PrePrepare(ctx) // prepare the Chrome instance just in case
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return nil, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	wifiConn, mtbferr := wifi.NewConn(ctx, s.CR, false, "", "", "", "")
	if mtbferr != nil {
		return nil, mtbferr
	}
	defer wifiConn.Close(false)

	status, mtbferr := wifiConn.CheckWifi(req.Enable)
	if mtbferr != nil {
		return nil, mtbferr
	}

	testing.ContextLog(ctx, "WifiService.Verify succeeded")

	return &ws.VerifyResponse{Status: status}, nil
}
