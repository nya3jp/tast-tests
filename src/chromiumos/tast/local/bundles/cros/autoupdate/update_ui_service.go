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
type UpdateUIService struct {
	s *testing.ServiceState

	cr *chrome.Chrome
}

// New logs into Chrome with default options.
// Unless the device is restarted in the meantime, call Close to clean up resources.
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

// Close closes the Chrome instance.
func (u *UpdateUIService) Close(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if u.cr == nil {
		return nil, errors.New("Chrome instance doesn't exists")
	}
	err := u.cr.Close(ctx)
	u.cr = nil
	return &empty.Empty{}, err
}

// RelaunchAfterUpdate clicks the "Restart" button in the settings page. (Only available if there's an update pending.)
func (u *UpdateUIService) RelaunchAfterUpdate(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if u.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	tconn, err := u.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to test API")
	}

	restart := nodewith.Name("Restart").Role(role.Button)
	ui := uiauto.New(tconn)

	if _, err := ossettings.LaunchAtPageURL(ctx, tconn, u.cr, "help/about", ui.Exists(restart)); err != nil {
		return nil, errors.Wrap(err, "failed to launch about ChromeOS settings")
	}

	// This will restart the system and thus also terminate the current Chrome instance.
	if err := ui.LeftClick(restart)(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click restart button")
	}

	return &empty.Empty{}, nil
}
