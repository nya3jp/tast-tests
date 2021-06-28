// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
