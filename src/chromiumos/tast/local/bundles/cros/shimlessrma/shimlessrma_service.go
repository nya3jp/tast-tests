// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shimlessrma

import (
	"context"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/shimlessrmaapp"
	pb "chromiumos/tast/services/cros/shimlessrma"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			pb.RegisterAppServiceServer(srv, &AppService{s: s})
		},
	})
}

// AppService contains context about shimless rma.
type AppService struct {
	s   *testing.ServiceState
	cr  *chrome.Chrome
	app *shimlessrmaapp.RMAApp
}

// NewShimlessRMA creates ShimlessRMA.
func (shimlessRMA *AppService) NewShimlessRMA(ctx context.Context, req *pb.NewShimlessRMARequest) (*empty.Empty, error) {
	// Make sure rmad is not currently running.
	if err := testexec.CommandContext(ctx, "stop", "rmad").Run(); err != nil {
		return nil, errors.Wrap(err, "failed to stop rmad")
	}

	// Create a valid empty rmad state file.
	if err := shimlessrmaapp.CreateEmptyStateFile(); err != nil {
		return nil, errors.Wrap(err, "failed to create rmad state file")
	}

	cr, err := chrome.New(ctx, chrome.EnableFeatures("ShimlessRMAFlow"),
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(req.ManifestKey),
		chrome.ExtraArgs("--launch-rma"))
	if err != nil {
		return nil, errors.Wrap(err, "Fail to new Chrome")
	}

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect Test API")
	}

	app, err := shimlessrmaapp.App(ctx, tconn)
	if err != nil {
		return nil, errors.Wrap(err, "failed to launch Shimless RMA app")
	}

	shimlessRMA.cr = cr
	shimlessRMA.app = app

	return &empty.Empty{}, nil
}

// CloseShimlessRMA closes and releases the resources obtained by New.
func (shimlessRMA *AppService) CloseShimlessRMA(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	if err := shimlessrmaapp.RemoveStateFile(); err != nil {
		return nil, errors.Wrap(err, "failed to remote state file")
	}

	if err := shimlessRMA.cr.Close(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to close Shimless RMA app")
	}

	return &empty.Empty{}, nil
}

// TestWelcomeAndCancel tests welcome page is loaded and then cancel it.
func (shimlessRMA *AppService) TestWelcomeAndCancel(ctx context.Context, rreq *empty.Empty) (*empty.Empty, error) {
	// TODO: refactor testing steps to separate method later.
	if err := shimlessRMA.app.WaitForWelcomePageToLoad()(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to load welcome page")
	}

	if err := shimlessRMA.app.LeftClickCancelButton()(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click cancel button")
	}

	if err := shimlessRMA.app.WaitForStateFileDeleted()(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to cancel RMA, state file not deleted")
	}

	if err := testexec.CommandContext(ctx, "stop", "rmad").Run(); err != nil {
		return nil, errors.Wrap(err, "failed to stop RMA")
	}

	return &empty.Empty{}, nil
}
