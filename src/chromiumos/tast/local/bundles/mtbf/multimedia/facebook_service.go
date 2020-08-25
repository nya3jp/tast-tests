// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package multimedia

import (
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/common/mtbferrors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui"
	mtbfchrome "chromiumos/tast/local/mtbf/chrome"
	"chromiumos/tast/services/mtbf/video"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			video.RegisterFacebookServiceServer(srv, &FacebookService{chrome.SvcLoginReusePre{S: s}})
		},
	})
}

// FacebookService implements tast.mtbf.video.FacebookService.
type FacebookService struct {
	chrome.SvcLoginReusePre
}

// OpenFacebook opens Facebook.
func (s *FacebookService) OpenFacebook(ctx context.Context, req *video.OpenFacebookRequest) (*empty.Empty, error) {

	testing.ContextLog(ctx, "FacebookService.OpenFacebook called")

	err := s.PrePrepare(ctx) // prepare the Chrome instance just in case
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.GRPCPrePrepare, err)
	}

	if s.CR == nil {
		return nil, mtbferrors.New(mtbferrors.ChromeInst, nil)
	}

	conn, mtbferr := mtbfchrome.NewConn(ctx, s.CR, req.IntentURL)
	if mtbferr != nil {
		return nil, mtbferr
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	tconn, err := s.CR.TestAPIConn(ctx)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.ChromeTestConn, err)
	}

	openBtn, err := ui.FindWithTimeout(ctx, tconn, ui.FindParams{Role: ui.RoleTypeButton, Name: "Open"}, time.Minute)
	if err != nil {
		return nil, mtbferrors.New(mtbferrors.VideoWaitAndClick, err, "Open button")
	}
	defer openBtn.Release(ctx)

	testing.Sleep(ctx, 1*time.Second)
	if err := openBtn.LeftClick(ctx); err != nil {
		return nil, mtbferrors.New(mtbferrors.VideoWaitAndClick, err, "Open button")
	}
	testing.ContextLog(ctx, "FacebookService.OpenFacebook succeeded")

	return &empty.Empty{}, nil
}
