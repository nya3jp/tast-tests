// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package inputs

import (
	"context"
	"fmt"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/inputs"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			inputs.RegisterKeyboardServiceServer(srv, &KeyboardService{s: s})
		},
	})
}

// KeyboardService implements tast.cros.inputs.KeyboardService.
type KeyboardService struct {
	s *testing.ServiceState

	cr    *chrome.Chrome
	tconn *chrome.TestConn
}

// Type allows uses to input keystrokes
func (ts *KeyboardService) Type(ctx context.Context, req *inputs.TypeRequest) (*empty.Empty, error) {
	if ts.cr == nil {
		cr, err := chrome.New(ctx)
		if err != nil {
			return nil, err
		}
		ts.cr = cr

		tconn, err := cr.TestAPIConn(ctx)
		if err != nil {
			testing.ContextLog(ctx, "Failed to get a connection to the Test Extension")
			return nil, err
		}
		ts.tconn = tconn
	}

	if _, err := ts.cr.NewConn(ctx, fmt.Sprintf("https://google.com/search?q=%s", req.Key)); err != nil {
		return nil, err
	}

	return &empty.Empty{}, nil
}
