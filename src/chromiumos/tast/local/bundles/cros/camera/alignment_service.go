// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"io/ioutil"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"

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
	outDir string
	http   *httptest.Server
	s      *testing.ServiceState
	cr     *chrome.Chrome
	conn   *chrome.Conn
}

func (a *AlignmentService) Prepare(ctx context.Context, req *pb.PrepareRequest) (_ *empty.Empty, retErr error) {
	defer func() {
		if retErr != nil {
			a.cleanup(ctx)
		}
	}()

	a.outDir = req.OutDir
	a.http = httptest.NewServer(http.FileServer(http.Dir(a.outDir)))

	cr, err := chrome.New(ctx, chrome.ARCDisabled(), chrome.Auth(req.Username, req.Password, ""), chrome.GAIALogin(), chrome.KeepState())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}
	a.cr = cr
	defer func() {
		if retErr != nil {
			a.cr.Close(ctx)
			a.cr = nil
		}
	}()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}

	// TODO(b/166370953): Handle CRD timeout.
	if err := crd.PrepareCRD(ctx, cr, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to PrepareCRD")
	}

	testing.ContextLog(ctx, "Waiting connection")
	if err := crd.WaitConnection(ctx, tconn); err != nil {
		return nil, errors.Wrap(err, "no client connected")
	}

	return &empty.Empty{}, nil
}

func (a *AlignmentService) GetPreviewFrame(ctx context.Context, req *pb.GetPreviewFrameRequest) (*empty.Empty, error) {
	if a.conn == nil {
		conn, err := a.cr.NewConn(ctx, a.http.URL+"/preview.html")
		if err != nil {
			return nil, errors.Wrap(err, "failed to open preview.html")
		}
		a.conn = conn
	}

	// TODO(b/166370953): Auto allow "... wants to Use your camera" dialog.
	var frame []byte
	if err := a.conn.Call(ctx, &frame, "getPreviewFrame", req.Facing, req.Ratio); err != nil {
		return nil, errors.Wrapf(err, "failed to open preview %v facing, %v aspectRatio", req.Facing, req.Ratio)
	}

	if err := ioutil.WriteFile(filepath.Join(a.outDir, "frame.jpg"), frame, 0644); err != nil {
		return nil, errors.Wrap(err, "failed to write preview frame.jpg")
	}
	return &empty.Empty{}, nil
}

func (a *AlignmentService) FeedbackAlign(ctx context.Context, req *pb.FeedbackAlignRequest) (*empty.Empty, error) {
	if a.conn == nil {
		return nil, errors.New("cannot FeedbackAlign() before CheckAlign()")
	}

	if err := a.conn.Call(ctx, nil, "feedbackAlign", req.Passed, req.Msg); err != nil {
		return nil, errors.Wrap(err, "failed to call feedbackAlign()")
	}
	return &empty.Empty{}, nil
}

func (a *AlignmentService) cleanup(ctx context.Context) {
	if a.conn != nil {
		a.conn.CloseTarget(ctx)
		a.conn.Close()
		a.conn = nil
	}
	if a.cr != nil {
		a.cr.Close(ctx)
		a.cr = nil
	}
	if a.http != nil {
		a.http.Close()
		a.http = nil
	}
	if len(a.outDir) > 0 {
		os.RemoveAll(a.outDir)
		a.outDir = ""
	}
}

func (a *AlignmentService) Cleanup(ctx context.Context, _ *empty.Empty) (*empty.Empty, error) {
	a.cleanup(ctx)
	return &empty.Empty{}, nil
}
