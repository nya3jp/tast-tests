// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package bluetooth

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"
	"google.golang.org/grpc/codes"
	"google.golang.org/grpc/status"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/bundles/mtbf/bluetooth/btservice"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mtbf/service"
	"chromiumos/tast/services/mtbf/bluetooth"
	"chromiumos/tast/testing"
)

const joinBtn = `document.querySelector("#yDmH0d > div.WOi1Wb > div.GhN39b > div > div > div > div > div > span")`

type Testcases struct {
	service.Service
}

func run039JoinHangout(ctx context.Context, cr *chrome.Chrome, hangoutURL string) error {
	conn, err := cr.NewConn(ctx, hangoutURL)
	if err != nil {
		return mtbferrors.New(mtbferrors.ChromeJoinHangout, err, hangoutURL)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	testing.ContextLog(ctx, "joinBtn: ", joinBtn)
	testing.Sleep(ctx, 3*time.Second)

	if err := conn.WaitForExprWithTimeout(ctx, joinBtn, 30*time.Second); err != nil {
		return mtbferrors.New(mtbferrors.ChromeJoinHangout, err, hangoutURL)
	}

	testing.Sleep(ctx, 3*time.Second)

	if err := conn.Exec(ctx, joinBtn+".click()"); err != nil {
		return mtbferrors.New(mtbferrors.ChromeJoinHangout, err, hangoutURL)
	}

	testing.Sleep(ctx, 10*time.Second)
	return nil
}

func (s *Testcases) RunLocal039Part1(ctx context.Context, req *bluetooth.Case039Request) (*empty.Empty, error) {
	if req.A2DPDeviceName == "" || req.HangoutsURL == "" {
		return nil, status.Error(codes.InvalidArgument, "A2DP_device_name or hangoutsURL is empty")
	}
	testing.ContextLog(ctx, "Testcases: run MTBF 039 local case")
	conn, err := s.NewConn(ctx, chrome.BlankURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	btConn, err := btservice.New(ctx, s.CR)

	if err != nil {
		return &empty.Empty{}, err
	}
	defer btConn.Close()
	testing.ContextLog(ctx, "btDeviceName: ", req.A2DPDeviceName)
	btAddress, err := btConn.GetAddress(req.A2DPDeviceName)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "btAddress: ", btAddress)
	btConsole, err := btservice.NewConsole(ctx, s.CR)
	if err != nil {
		return nil, err
	}
	defer btConsole.Close(ctx)

	if err := btConsole.Connect(ctx, btAddress); err != nil {
		return nil, err
	}

	_, err = btConsole.CheckScanning(ctx, true)
	if err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}
func (s *Testcases) RunLocal039Part2(ctx context.Context, req *bluetooth.Case039Request) (*empty.Empty, error) {
	conn, err := s.NewConn(ctx, chrome.BlankURL)
	if err != nil {
		return nil, err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	btConn, err := btservice.New(ctx, s.CR)
	if err != nil {
		return nil, err
	}
	defer btConn.Close()

	btConsole, err := btservice.NewConsole(ctx, s.CR)
	if err != nil {
		return nil, err
	}
	defer btConsole.Close(ctx)

	testing.ContextLog(ctx, "btDeviceName: ", req.A2DPDeviceName)
	btAddress, err := btConn.GetAddress(req.A2DPDeviceName)
	if err != nil {
		return nil, err
	}

	isA2dp, err := btConsole.IsA2dp(ctx, btAddress)
	if err != nil {
		return nil, err
	}

	testing.ContextLog(ctx, "isA2dp: ", isA2dp)
	if !isA2dp {
		return nil, mtbferrors.New(mtbferrors.BTNotA2DP, nil, req.A2DPDeviceName)
	}

	return &empty.Empty{}, nil
}
func (s *Testcases) RunLocal039Part3(ctx context.Context, req *bluetooth.Case039Request) (*empty.Empty, error) {
	conn, err := s.NewConn(ctx, chrome.BlankURL)
	if err != nil {
		return &empty.Empty{}, err
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	btConn, err := btservice.New(ctx, s.CR)
	if err != nil {
		return &empty.Empty{}, err
	}
	defer btConn.Close()

	btConsole, err := btservice.NewConsole(ctx, s.CR)
	if err != nil {
		return &empty.Empty{}, err
	}
	defer btConsole.Close(ctx)

	testing.ContextLog(ctx, "btDeviceName: ", req.A2DPDeviceName)
	btAddress, err := btConn.GetAddress(req.A2DPDeviceName)
	if err != nil {
		return &empty.Empty{}, err
	}

	isHsp, err := btConsole.IsHsp(ctx, btAddress)
	if err != nil {
		return &empty.Empty{}, err
	}

	testing.ContextLog(ctx, "Joined hangout. isHsp: ", isHsp)
	if !isHsp {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.BTNotHSP, nil, req.A2DPDeviceName)
	}

	if connected, err := btConn.CheckConnectedByAddr(btAddress); err != nil {
		return &empty.Empty{}, err
	} else if !connected {
		return &empty.Empty{}, mtbferrors.New(mtbferrors.BTConnectFailed, nil, btAddress)
	}

	return &empty.Empty{}, nil
}

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			bluetooth.RegisterTestcasesServer(srv, &Testcases{service.New(s)})
		},
	})
}
