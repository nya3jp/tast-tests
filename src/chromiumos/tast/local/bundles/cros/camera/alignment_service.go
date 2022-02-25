// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/fsutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/crd"
	"chromiumos/tast/local/cryptohome"
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
		chrome.GAIALogin(chrome.Creds{User: req.Username, Pass: req.Password}),
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
	if err := crd.Launch(ctx, cr.Browser(), tconn); err != nil {
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

// copyPreviewFrame copies the last preview frame image from download folder for debugging.
func (a *AlignmentService) copyPreviewFrame(ctx context.Context, cr *chrome.Chrome) error {
	userPath, err := cryptohome.UserPath(ctx, cr.NormalizedUser())
	if err != nil {
		return errors.Wrap(err, "failed to get the cryptohome user path")
	}
	const frameFileName = "frame.png"
	framePath := filepath.Join(userPath, "MyFiles", "Downloads", frameFileName)
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("failed to get remote context dir")
	}
	if err := fsutil.CopyFile(framePath, filepath.Join(outDir, frameFileName)); err != nil {
		return errors.Wrap(err, "failed copy frame image to out dir")
	}
	return nil
}

func (a *AlignmentService) CheckRegression(ctx context.Context, req *pb.CheckRegressionRequest) (*pb.CheckRegressionResponse, error) {
	srv := httptest.NewServer(http.FileServer(http.Dir(req.DataPath)))
	defer srv.Close()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.ARCDisabled(),
		// Avoid the need to grant camera/microphone permissions.
		chrome.ExtraArgs("--use-fake-ui-for-media-stream"))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}
	defer cr.Close(cleanupCtx)

	conn, err := cr.NewConn(ctx, srv.URL+"/camerabox_align.html")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open camerabox_align.html")
	}
	defer conn.Close()
	defer conn.CloseTarget(cleanupCtx)

	if err := conn.Call(ctx, nil, "Tast.checkRegression", req.Facing); err != nil {
		if err := a.copyPreviewFrame(cleanupCtx, cr); err != nil {
			testing.ContextLog(ctx, "Failed to copy last preview frame: ", err)
		}
		return &pb.CheckRegressionResponse{
			Result: pb.TestResult_TEST_RESULT_FAILED,
			Error:  err.Error(),
		}, nil
	}

	return &pb.CheckRegressionResponse{Result: pb.TestResult_TEST_RESULT_PASSED}, nil
}
