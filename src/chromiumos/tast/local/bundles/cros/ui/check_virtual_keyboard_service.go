// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package ui

import (
	"context"
	"path/filepath"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/ui"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterCheckVirtualKeyboardServiceServer(srv, &CheckVirtualKeyboardService{
				sharedObject: common.SharedObjectsForServiceSingleton,
			})
		},
	})
}

// CheckVirtualKeyboardService implements the methods defined in CheckVirtualKeyboardServiceServer.
type CheckVirtualKeyboardService struct {
	cr           *chrome.Chrome
	br           *browser.Browser
	closeBrowser func(ctx context.Context)
	tconn        *chrome.TestConn // from ash-chrome
	sharedObject *common.SharedObjectsForService
}

// NewChromeLoggedIn Logs into a user session.
func (cvk *CheckVirtualKeyboardService) NewChromeLoggedIn(ctx context.Context, req *pb.NewBrowserRequest) (*empty.Empty, error) {
	if cvk.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	bt := browser.TypeLacros
	cfg := browserfixt.DefaultLacrosConfig
	if req.BrowserType == pb.NewBrowserRequest_ASH {
		bt = browser.TypeAsh
		cfg = nil
	}
	cr, br, closeBrowser, err := browserfixt.SetUpWithNewChrome(ctx, bt, cfg)
	if err != nil {
		return nil, err
	}

	cvk.cr = cr
	cvk.br = br
	cvk.closeBrowser = closeBrowser
	tconn, err := cvk.cr.TestAPIConn(ctx)
	if err != nil {
		return nil, err
	}
	cvk.tconn = tconn
	// Store the newly created chrome in the shared object so UtilsService or other services can use it.
	cvk.sharedObject.ChromeMutex.Lock()
	defer cvk.sharedObject.ChromeMutex.Unlock()
	cvk.sharedObject.Chrome = cr

	return &empty.Empty{}, nil
}

// OpenChromePage opens a chrome page.
func (cvk *CheckVirtualKeyboardService) OpenChromePage(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if cvk.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	// Open an empty page.
	conn, err := cvk.br.NewConn(ctx, "chrome://newtab/")
	if err != nil {
		return nil, errors.Wrap(err, "failed to open empty Chrome page")
	}
	defer conn.Close()

	return &empty.Empty{}, nil
}

// TouchChromeAddressBar uses touch screen to send a tap on the address bar.
func (cvk *CheckVirtualKeyboardService) TouchChromeAddressBar(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
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

// ClickChromeAddressBar sends a left click on the address bar.
func (cvk *CheckVirtualKeyboardService) ClickChromeAddressBar(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	uiauto := uiauto.New(cvk.tconn)
	addressBarNode := nodewith.Role(role.TextField).Name("Address and search bar")
	if err := uiauto.LeftClickUntil(
		addressBarNode,
		uiauto.WaitUntilExists(addressBarNode.Focused()),
	)(ctx); err != nil {
		return nil, errors.Wrap(err, "could not find the address bar")
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

	vkNode := nodewith.Name("Chrome OS Virtual Keyboard").Role(role.Keyboard).Visible()
	if err := uiauto.WithTimeout(3 * time.Second).WaitUntilExists(vkNode)(ctx); err != nil {
		if !req.IsDutTabletMode {
			return &pb.CheckVirtualKeyboardResponse{
				IsVirtualKeyboardPresent: exists,
			}, nil
		}
		saveLogs(ctx, cvk, func() bool { return true }, "check")
		return nil, errors.Wrap(err, "unable to detect Chrome OS virtual keyboard")
	}

	exists = true
	return &pb.CheckVirtualKeyboardResponse{IsVirtualKeyboardPresent: exists}, nil
}

// CloseChrome closes a Chrome session and cleans up the resources obtained by NewChrome.
func (cvk *CheckVirtualKeyboardService) CloseChrome(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if cvk.closeBrowser != nil {
		cvk.closeBrowser(ctx)
		cvk.closeBrowser = nil
	}
	if cvk.cr == nil {
		return nil, errors.New("Chrome not available")
	}
	err := cvk.cr.Close(ctx)
	cvk.cr = nil
	return &empty.Empty{}, err
}

func saveLogs(ctx context.Context, cvk *CheckVirtualKeyboardService, hasError func() bool, msg string) error {
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return errors.New("no output directory")
	}
	faillog.DumpUITreeOnError(ctx, filepath.Join(outDir, "CheckVirtualKeyboardService-"+msg), hasError, cvk.tconn)
	faillog.SaveScreenshotOnError(ctx, cvk.cr, filepath.Join(outDir, "CheckVirtualKeyboardService-"+msg), hasError)
	return nil
}
