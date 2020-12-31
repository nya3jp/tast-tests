// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"net/http"
	"net/http/httptest"

	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/crd"
	pb "chromiumos/tast/services/cros/camerabox"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterAlignmentServiceServer(srv, &AlignmentService{s: s})
		},
	})
}

type AlignmentService struct {
	s *testing.ServiceState
}

func (a *AlignmentService) ManualAlign(ctx context.Context, req *pb.ManualAlignRequest) (*empty.Empty, error) {
	srv := httptest.NewServer(http.FileServer(http.Dir(req.DataPath)))
	defer srv.Close()

	cr, err := chrome.New(ctx, chrome.ARCDisabled(),
		chrome.Auth(req.Username, req.Password, ""), chrome.GAIALogin(),
		chrome.KeepState(),
		// Avoid the need to grant camera/microphone permissions.
		chrome.ExtraArgs("--use-fake-ui-for-media-stream"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}
	// TODO(b/166370953): Handle CRD timeout.
	if err := crd.Launch(ctx, cr, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to launch remote desktop")
	}
	testing.ContextLog(ctx, "Waiting connection")
	if err := crd.WaitConnection(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "no client connected")
	}

	conn, err := cr.NewConn(ctx, srv.URL+"/camerabox_align.html")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open camerabox_align.html")
	}
	defer func() {
		conn.CloseTarget(ctx)
		conn.Close()
	}()

	if err := conn.Call(ctx, nil, "Tast.manualAlign", req.Facing); err != nil {
		return nil, errors.Wrap(err, "failed to call manualAlign on camerabox_align.html")
	}

	return &empty.Empty{}, nil
}
