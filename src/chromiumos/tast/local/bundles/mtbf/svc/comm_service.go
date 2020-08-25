// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package svc

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/faillog"
	"chromiumos/tast/services/mtbf/svc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			svc.RegisterCommServiceServer(srv,
				&CommService{
					chrome.SvcLoginReusePre{S: s},
					&arc.SvcLoginReusePre{},
				})
		},
	})
}

// CommService implements tast.mtbf.svc.CommService.
type CommService struct {
	// embedded structure to get the implementation of LoginReusePre
	chrome.SvcLoginReusePre

	// svcArcPre is the instance of ARC SvcLoginReusePre that holds the ARC information.
	svcArcPre *arc.SvcLoginReusePre
}

// Login does Chrome Login.
func (c *CommService) Login(ctx context.Context, req *svc.LoginRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "CommService - Login called. OutDir - ", req.OutDir)
	var err error
	res := &empty.Empty{}

	if _, err = c.MakeOutDir(ctx, req.OutDir); err != nil {
		return res, mtbferrors.New(mtbferrors.GRPCOutDir, err)
	}

	if err = c.PrePrepare(ctx); err != nil {
		//testing.ContextLog(ctx, "Login Failed. Restarting UI before failing...")
		//upstart.RestartJob(ctx, "ui")
		return res, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}
	return res, nil

}

// LoginWithARC does Chrome and ARC login.
func (c *CommService) LoginWithARC(ctx context.Context, req *svc.LoginRequest) (*empty.Empty, error) {
	testing.ContextLog(ctx, "CommService - LoginWithARC called. OutDir - ", req.OutDir)
	var err error
	res := &empty.Empty{}

	if _, err = c.MakeOutDir(ctx, req.OutDir); err != nil {
		return res, mtbferrors.New(mtbferrors.GRPCOutDir, err)
	}

	if err = c.PrePrepare(ctx); err != nil {
		//testing.ContextLog(ctx, "Login Failed. Restarting UI before failing...")
		//upstart.RestartJob(ctx, "ui")
		return res, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if c.svcArcPre, err = arc.NewForRPC(ctx, c.OutDir()); err != nil {
		return res, mtbferrors.New(mtbferrors.GRPCLoginARC, err)
	}

	return res, nil
}

// Close does Chrome and ARC cleanup.
func (c *CommService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	testing.ContextLog(ctx, "CommService - Close called")
	res := &empty.Empty{}

	c.svcArcPre.Close()

	if err := c.PreClose(ctx); err != nil {
		return res, mtbferrors.New(mtbferrors.GRPCLoginClose, err)
	}

	return res, nil
}

// TakeScreenshot takes screenshots.
func (c *CommService) TakeScreenshot(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	testing.ContextLog(ctx, "CommService - TakeScreenshot called")
	dir := "/home/chronos/user/Downloads"
	faillog.SaveScreenshot(ctx, dir, "")

	return &empty.Empty{}, nil
}
