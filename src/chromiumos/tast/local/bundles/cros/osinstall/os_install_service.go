// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package osinstall

import (
	"context"
	"strings"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/lsbrelease"
	"chromiumos/tast/services/cros/osinstall"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			osinstall.RegisterOsInstallServiceServer(srv, &osInstallService{s: s})
		},
	})
}

type osInstallService struct {
	s      *testing.ServiceState
	cr     *chrome.Chrome
	tconn  *chrome.TestConn
	ui     *uiauto.Context
	outDir string
}

func (svc *osInstallService) StartChrome(ctx context.Context, req *osinstall.StartChromeRequest) (*empty.Empty, error) {
	if svc.cr != nil {
		return nil, errors.New("Chrome already available")
	}

	// Start Chrome but don't log in. Allow a special extension for
	// interacting with the UI during OOBE.
	allowProfileExtension := chrome.LoadSigninProfileExtension(req.SigninProfileTestExtensionID)
	cr, err := chrome.New(ctx, chrome.NoLogin(), allowProfileExtension)
	if err != nil {
		return nil, err
	}

	// Create the connection that allows us to manipulate the UI.
	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	// Get output dir in which to store UI dump.
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get remote output directory")
	}

	svc.cr = cr
	svc.tconn = tconn
	svc.ui = uiauto.New(tconn)
	svc.outDir = outDir

	return &empty.Empty{}, nil
}

func (svc *osInstallService) DumpUITree(ctx context.Context) {
	faillog.DumpUITree(ctx, svc.outDir, svc.tconn)
}

func (svc *osInstallService) RunOsInstall(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	ui := svc.ui

	// Advance to the install-or-try screen.
	if err := ui.LeftClick(nodewith.Name("Get started").Role(role.Button))(ctx); err != nil {
		svc.DumpUITree(ctx)
		return nil, err
	}

	// Ensure the install option is selected.
	if err := ui.LeftClick(nodewith.NameContaining("Install ChromeOS Flex").Role(role.RadioButton))(ctx); err != nil {
		svc.DumpUITree(ctx)
		return nil, err
	}

	// Advance to the install confirmation screen.
	if err := ui.LeftClick(nodewith.Name("Next").Role(role.Button))(ctx); err != nil {
		svc.DumpUITree(ctx)
		return nil, err
	}

	// Confirm readiness, which will bring up one final warning dialog.
	if err := ui.LeftClick(nodewith.Name("Install ChromeOS Flex").Role(role.Button))(ctx); err != nil {
		svc.DumpUITree(ctx)
		return nil, err
	}

	// Confirm readiness and start the install process.
	if err := ui.LeftClick(nodewith.Name("Install").Role(role.Button))(ctx); err != nil {
		svc.DumpUITree(ctx)
		return nil, err
	}

	// Wait for the screen shown during the install process.
	if err := ui.WaitUntilExists(nodewith.Name("Installing ChromeOS Flex").Role(role.Dialog))(ctx); err != nil {
		svc.DumpUITree(ctx)
		return nil, err
	}

	// The UI text says it can take up to 20 minutes to install. In
	// general it should be much faster (about 3 minutes in an ordinary
	// VM), but on older physical hardware it can indeed be very slow.
	maxInstallDuration := 20 * time.Minute

	// Wait for the install-complete screen.
	if err := ui.WithTimeout(maxInstallDuration).WaitUntilExists(nodewith.Name("Installation complete").Role(role.Dialog))(ctx); err != nil {
		svc.DumpUITree(ctx)
		return nil, err
	}

	return &empty.Empty{}, nil
}

func (svc *osInstallService) ShutDown(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := svc.ui.LeftClick(nodewith.Name("Shut down").Role(role.Button))(ctx); err != nil {
		svc.DumpUITree(ctx)
		return nil, err
	}

	return &empty.Empty{}, nil
}

func IsRunningFromInstaller(ctx context.Context) (bool, error) {
	outB, err := testexec.CommandContext(ctx, "is_running_from_installer").Output()
	if err != nil {
		return false, errors.Wrap(err, "is_running_from_installer failed")
	}
	out := strings.TrimSpace(string(outB))

	if out == "yes" {
		return true, nil
	} else if out == "no" {
		return false, nil
	} else {
		return false, errors.Errorf("invalid is_running_from_installer output: %s", out)
	}
}

func (*osInstallService) GetOsInfo(ctx context.Context, req *empty.Empty) (*osinstall.GetOsInfoResponse, error) {
	isRunningFromInstaller, err := IsRunningFromInstaller(ctx)
	if err != nil {
		return nil, err
	}

	lsbContent, err := lsbrelease.Load()
	if err != nil {
		return nil, err
	}

	return &osinstall.GetOsInfoResponse{
		IsRunningFromInstaller: isRunningFromInstaller,
		Version:                lsbContent[lsbrelease.Version],
	}, nil
}
