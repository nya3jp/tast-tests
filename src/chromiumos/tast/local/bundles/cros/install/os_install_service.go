// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package install

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/services/cros/install"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			install.RegisterOsInstallServiceServer(srv, &OsInstallService{s})
		},
	})
}

type OsInstallService struct {
	s *testing.ServiceState
}

func (*OsInstallService) StartOsInstall(ctx context.Context, req *install.StartOsInstallRequest) (*empty.Empty, error) {
	// Start Chrome but don't log in.
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.LoadSigninProfileExtension(req.SigninProfileTestExtensionID))
	if err != nil {
		return nil, err
	}

	// Create the connection that allows us to manipulate the UI.
	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, err
	}

	// Create a UI dump.
	outDir, ok := testing.ContextOutDir(ctx)
	if !ok {
		return nil, errors.New("failed to get remote output directory")
	}
	faillog.DumpUITree(ctx, outDir, tconn)

	// Intentionally fail the test for now.
	return nil, errors.New("TODO")
}
