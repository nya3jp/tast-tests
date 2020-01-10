// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package crash contains RPC wrappers to set up and tear down tests.
package crash

import (
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/crash"
	crash_service "chromiumos/tast/services/cros/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			crash_service.RegisterFixtureServiceServer(srv, &FixtureService{s: s})
		},
	})
}

// FixtureService implements tast.cros.crash.FixtureService
type FixtureService struct {
	s *testing.ServiceState

	cr *chrome.Chrome
}

func (c *FixtureService) SetUp(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if c.cr != nil {
		return nil, errors.New("already set up")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}
	c.cr = cr

	if err := crash.SetUpCrashTest(ctx, crash.WithConsent(cr)); err != nil {
		return nil, err
	}
	return &empty.Empty{}, nil
}

func (c *FixtureService) TearDown(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	var firstErr error
	if err := crash.TearDownCrashTest(); err != nil {
		firstErr = err
	}
	if c.cr != nil {
		// c.cr could be nil if the machine rebooted in the middle,
		// so don't complain if it is.
		if err := c.cr.Close(ctx); err != nil && firstErr == nil {
			firstErr = err
		}
		c.cr = nil
	}
	return &empty.Empty{}, firstErr
}
