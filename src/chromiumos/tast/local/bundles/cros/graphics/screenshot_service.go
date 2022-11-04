// Copyright 2021 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package graphics

import (
	"context"
	"os"
	"path/filepath"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/services/cros/graphics"
	pb "chromiumos/tast/services/cros/graphics"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			graphics.RegisterScreenshotServiceServer(srv, &ScreenshotService{s: s})
		},
	})
}

// ScreenshotService implements tast.cros.graphics.ScreenshotService.
type ScreenshotService struct {
	s *testing.ServiceState
}

// CaptureScreenAndDelete captures a temporary screenshot, and deletes it immediately.
func (s *ScreenshotService) CaptureScreenAndDelete(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		return nil, errors.New("output directory unavailable")
	}
	path := filepath.Join(dir, "screenshotTest.png")
	defer os.Remove(path)
	if err := screenshot.CaptureWithStderr(ctx, path); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

// CaptureScreenshot captures a screenshot and saves it in the output directory of the test under filePrefix.png.
func (s *ScreenshotService) CaptureScreenshot(ctx context.Context, req *pb.CaptureScreenshotRequest) (*empty.Empty, error) {
	dir, ok := testing.ContextOutDir(ctx)
	if !ok || dir == "" {
		return nil, errors.New("output directory unavailable")
	}
	path := filepath.Join(dir, req.FilePrefix+".png")

	testing.ContextLog(ctx, "Capturing screenshot at ", path)

	if err := screenshot.Capture(ctx, path); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}
