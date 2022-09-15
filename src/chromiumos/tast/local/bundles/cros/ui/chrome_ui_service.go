// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/common"
	"chromiumos/tast/local/upstart"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	var chromeUIService ChromeUIService
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			chromeUIService = ChromeUIService{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterChromeUIServiceServer(srv, &chromeUIService)
		},
	})
}

// ChromeUIService implements the methods defined in ChromeUIServiceServer.
type ChromeUIService struct {
	sharedObject *common.SharedObjectsForService
}

// EnsureLoginScreen emulates log out, and ensures login screen.
func (c *ChromeUIService) EnsureLoginScreen(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to restart ui job")
	}
	return &empty.Empty{}, nil
}

// DumpUITree dumps the UI tree to the context output directory of the test.
func (c *ChromeUIService) DumpUITree(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	cr := c.sharedObject.Chrome
	if cr == nil {
		return &empty.Empty{}, errors.New("Chrome has not been started")
	}
	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return &empty.Empty{}, errors.Wrap(err, "failed to create test API connection")
	}
	contextOutDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return &empty.Empty{}, errors.New("failed to get the context output directory")
	}
	faillog.DumpUITree(ctx, contextOutDir, tconn)
	return &empty.Empty{}, nil
}
