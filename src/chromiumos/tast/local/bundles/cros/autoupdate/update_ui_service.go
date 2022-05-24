// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package autoupdate

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/ossettings"
	"chromiumos/tast/local/chrome/uiauto/role"
	aupb "chromiumos/tast/services/cros/autoupdate"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			aupb.RegisterUpdateUIServiceServer(srv, &UpdateUIService{s: s})
		},
	})
}

// UpdateUIService implements tast.cros.autoupdate.UpdateUIService.
type UpdateUIService struct { // NOLINT
	s *testing.ServiceState

	cr *chrome.Chrome
}

func (u *UpdateUIService) New(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if u.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}

	u.cr = cr

	return &empty.Empty{}, nil
}

func (u *UpdateUIService) RelaunchAfterUpdate(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if u.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	tconn, err := u.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	_, err = ossettings.LaunchAtPage(ctx, tconn, ossettings.AboutChromeOS)
	if err != nil {
		return nil, err
	}

	// This will restart the system and thus, also terminate the current Chrome instance.
	restart := nodewith.Name("Restart").Role(role.Button)

	ui := uiauto.New(tconn)
	ui.WaitUntilExists(restart)(ctx)
	ui.LeftClick(restart)(ctx)

	return &empty.Empty{}, nil
}
