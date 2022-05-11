// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package shimlessrma

import (
	"context"
	"time"

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
func (shimlessRMA *AppService) NewShimlessRMA(ctx context.Context,
	req *pb.NewShimlessRMARequest) (*empty.Empty, error) {

	// If Reconnect is true, it means UI restarting during Shimless RMA testing.
	// Then, we don't need to stop rmad or create empty state file.
	if !req.Reconnect {
		// Make sure rmad is not currently running.
		// Ignore the error since ramd may not run at all.
		testexec.CommandContext(ctx, "stop", "rmad").Run()

		// Create a valid empty rmad state file.
		if err := shimlessrmaapp.CreateEmptyStateFile(); err != nil {
			return nil, errors.Wrap(err, "failed to create rmad state file")
		}
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
func (shimlessRMA *AppService) CloseShimlessRMA(ctx context.Context,
	req *empty.Empty) (*empty.Empty, error) {
	// Ignore failure handle in this method,
	// since we want to execute all of these anyway.
	shimlessRMA.app.WaitForStateFileDeleted()(ctx)

	testexec.CommandContext(ctx, "stop", "rmad").Run()

	shimlessrmaapp.RemoveStateFile()

	shimlessRMA.cr.Close(ctx)

	return &empty.Empty{}, nil
}

// TestWelcomeAndCancel tests welcome page is loaded and then cancel it.
func (shimlessRMA *AppService) TestWelcomeAndCancel(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {
	// TODO: refactor testing steps to separate method later.
	if err := shimlessRMA.app.WaitForWelcomePageToLoad()(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to load welcome page")
	}

	if err := shimlessRMA.app.LeftClickCancelButton()(ctx); err != nil {
		return nil, errors.Wrap(err, "failed to click cancel button")
	}

	return &empty.Empty{}, nil
}

// WaitForPageToLoad waits the page with title to be loaded.
func (shimlessRMA *AppService) WaitForPageToLoad(ctx context.Context,
	req *pb.WaitForPageToLoadRequest) (*empty.Empty, error) {
	waitTimeout := time.Duration(req.DurationInSecond) * time.Second
	if err := shimlessRMA.app.WaitForPageToLoad(req.Title, waitTimeout)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to load page: %s", req.Title)
	}

	return &empty.Empty{}, nil
}

// LeftClickButton left clicks the button with label.
func (shimlessRMA *AppService) LeftClickButton(ctx context.Context,
	req *pb.LeftClickButtonRequest) (*empty.Empty, error) {
	if err := shimlessRMA.app.LeftClickButton(req.Label)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to left click button: %s", req.Label)
	}

	return &empty.Empty{}, nil
}

// WaitUntilButtonEnabled waits for button with label eanbled.
func (shimlessRMA *AppService) WaitUntilButtonEnabled(ctx context.Context,
	req *pb.WaitUntilButtonEnabledRequest) (*empty.Empty, error) {
	waitTimeout := time.Duration(req.DurationInSecond) * time.Second
	if err := shimlessRMA.app.WaitUntilButtonEnabled(req.Label, waitTimeout)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to left click button: %s", req.Label)
	}

	return &empty.Empty{}, nil
}

// LeftClickRadioButton clicks radio button.
func (shimlessRMA *AppService) LeftClickRadioButton(ctx context.Context,
	req *pb.LeftClickRadioButtonRequest) (*empty.Empty, error) {
	if err := shimlessRMA.app.LeftClickRadioButton(req.Label)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to left click radio button: %s", req.Label)
	}

	return &empty.Empty{}, nil
}

// LeftClickLink clicks link.
func (shimlessRMA *AppService) LeftClickLink(ctx context.Context,
	req *pb.LeftClickLinkRequest) (*empty.Empty, error) {
	if err := shimlessRMA.app.LeftClickLink(req.Label)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to left click link: %s", req.Label)
	}

	return &empty.Empty{}, nil
}

// RetrieveTextByPrefix returns the text with prefix.
func (shimlessRMA *AppService) RetrieveTextByPrefix(ctx context.Context,
	req *pb.RetrieveTextByPrefixRequest) (*pb.RetrieveTextByPrefixResponse, error) {
	node, err := shimlessRMA.app.RetrieveTextByPrefix(ctx, req.Prefix)
	if err != nil {
		return nil, errors.Wrapf(err, "failed to find info with prefix: %s", req.Prefix)
	}

	return &pb.RetrieveTextByPrefixResponse{Value: node.Name}, nil
}

// EnterIntoTextInput enters content into text input.
func (shimlessRMA *AppService) EnterIntoTextInput(ctx context.Context,
	req *pb.EnterIntoTextInputRequest) (*empty.Empty, error) {

	if err := shimlessRMA.app.EnterIntoTextInput(ctx, req.TextInputName, req.Content)(ctx); err != nil {
		return nil, errors.Wrapf(err, "failed to enter content %s into text input", req.Content)
	}
	return &empty.Empty{}, nil
}
