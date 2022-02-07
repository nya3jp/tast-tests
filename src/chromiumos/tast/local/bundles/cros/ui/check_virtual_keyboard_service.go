// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterCheckVirtualKeyboardServiceServer(srv, &CheckVirtualKeyboardService{})
		},
	})
}

// CheckVirtualKeyboardService implements the methods defiend in CheckVirtualKeyboardServiceServer.
type CheckVirtualKeyboardService struct {
	cr    *chrome.Chrome
	tconn *chrome.TestConn
}

// NewChromeLoggedIn Logs into a user session.
func (cvk *CheckVirtualKeyboardService) NewChromeLoggedIn(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	if cvk.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	cr, err := chrome.New(ctx)
	if err != nil {
		return nil, err
	}

	cvk.cr = cr
	return &empty.Empty{}, nil
}

// OpenChromePage opens a chrome page.
func (cvk *CheckVirtualKeyboardService) OpenChromePage(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	if cvk.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	// Open an empty page.
	conn, err := cvk.cr.NewConn(ctx, "chrome://newtab/")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open empty Chrome page")
	}
	defer conn.Close()

	return &empty.Empty{}, nil
}

// ClickChromeAddressBar clicks on the address bar.
func (cvk *CheckVirtualKeyboardService) ClickChromeAddressBar(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	tconn, err := cvk.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	cvk.tconn = tconn

	addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar")
	tc, err := touch.New(ctx, cvk.tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create the touch context instance")
	}
	if err := tc.Tap(addressBarNode)(ctx); err != nil {
		return nil, errors.Wrap(err, "unable to detect Chrome OS virtual keyboard")
	}

	return &empty.Empty{}, nil
}

// CheckVirtualKeyboardIsPresent checks whether the virtual keyboard is present.
func (cvk *CheckVirtualKeyboardService) CheckVirtualKeyboardIsPresent(ctx context.Context, req *pb.CheckVirtualKeyboardRequest) (*pb.CheckVirtualKeyboardResponse, error) {

	if cvk.cr == nil {
		return nil, errors.New("Chrome not available")
	}

	var exists bool
	uiauto := uiauto.New(cvk.tconn)

	vkNode := nodewith.Name("Chrome OS Virtual Keyboard").Role(role.Keyboard).Onscreen()
	if err := uiauto.WithTimeout(3 * time.Second).WaitUntilExists(vkNode)(ctx); err != nil {
		if !req.IsDutTabletMode {
			return &pb.CheckVirtualKeyboardResponse{
				IsVirtualKeyboardPresent: exists,
			}, nil
		}
		return nil, errors.Wrap(err, "unable to detect Chrome OS virtual keyboard")
	}

	exists = true
	return &pb.CheckVirtualKeyboardResponse{IsVirtualKeyboardPresent: exists}, nil
}
