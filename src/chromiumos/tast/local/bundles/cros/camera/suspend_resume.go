// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package camera

import (
	"context"
	"io/ioutil"
	"os"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/ui"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/camera"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			camera.RegisterSuspendResumeServer(srv, &SuspendResume{s})
		},
	})
}

type SuspendResume struct {
	s *testing.ServiceState
}

func (*SuspendResume) SetUp(ctx context.Context, e *empty.Empty) (*empty.Empty, error) {
	out := new(empty.Empty)

	testing.ContextLog(ctx, "Setting up test environment")

	cr, err := chrome.New(ctx, chrome.ARCEnabled(), chrome.KeepState())
	if err != nil {
		return out, errors.Wrap(err, "failed to start Chrome")
	}
	defer cr.Close(ctx)

	td, err := ioutil.TempDir("", "")
	if err != nil {
		return nil, errors.Wrap(err, "failed to create a temp dir")
	}
	defer os.RemoveAll(td)
	a, err := arc.New(ctx, td)
	if err != nil {
		return out, errors.Wrap(err, "failed to start ARC")
	}
	defer a.Close()

	d, err := ui.NewDevice(ctx, a)
	if err != nil {
		return out, errors.Wrap(err, "failed to initialize UI Automator")
	}
	defer d.Close()

	return out, nil
}
