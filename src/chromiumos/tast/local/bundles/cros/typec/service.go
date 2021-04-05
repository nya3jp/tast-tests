// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package typec

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/typec/typecutils"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/services/cros/typec"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			typec.RegisterServiceServer(srv, &Service{s: s})
		},
	})
}

// Service implements tast.cros.typec.Service.
type Service struct {
	s  *testing.ServiceState
	cr *chrome.Chrome
}

// NewChromeLoginWithPeripheralDataAccess logs in to Chrome as a fake user, but before that, enables the DevicePciPeripheralDataAccess setting.
func (c *Service) NewChromeLoginWithPeripheralDataAccess(ctx context.Context, req *typec.KeyPath) (*empty.Empty, error) {
	// Get to the Chrome login screen.
	cr, err := chrome.New(ctx,
		chrome.DeferLogin())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start Chrome at login screen")
	}

	// Enable the setting.
	if err := typecutils.EnablePeripheralDataAccess(ctx, req.Path); err != nil {
		return nil, errors.Wrap(err, "failed to enable peripheral data access setting")
	}

	if err := cr.ContinueLogin(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to login")
	}

	c.cr = cr

	return &empty.Empty{}, nil
}
