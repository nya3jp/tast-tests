// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"fmt"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/common/testexec"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	arcpb "chromiumos/tast/services/cros/arc"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			arcpb.RegisterAdbSideloadServiceServer(srv, &AdbSideloadService{s: s})
		},
	})
}

type AdbSideloadService struct {
	s *testing.ServiceState
}

func (*AdbSideloadService) SetRequestAdbSideloadFlag(ctx context.Context, request *arcpb.SigninRequest) (*empty.Empty, error) {
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.KeepState(), chrome.LoadSigninProfileExtension(request.Key))
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "creating test API connection failed")
	}
	defer tconn.Close()

	// Adding the flag in Local State json
	// couldn't use tast.promisify here as we are using the TestAPIConn before the login has happened, and tast is not defined yet
	if err := tconn.Eval(ctx, `
	new Promise((resolve, reject) => {
		chrome.autotestPrivate.setWhitelistedPref('EnableAdbSideloadingRequested', true, () => {
			resolve();
		});
	})`, nil); err != nil {
		testing.ContextLog(ctx, "Error received while setting the sideloading flag: ", err)
	}

	// ImportantFileWriter flushes every 10 seconds. Wait for EnableAdbSideloadingRequested to be written before continuing.
	// TODO : Convert the polling function to an Explicit write to the DUT's disk
	testing.ContextLog(ctx, "Waiting for Enable ADB Sideloading flag to be written on DUT's Local State json")
	testing.Poll(ctx, func(ctx context.Context) error {
		if err := testexec.CommandContext(ctx, "grep", "-c", "'EnableAdbSideloadingRequested'", "/home/chronos/Local State").Run(); err != nil {
			return err
		}
		return nil
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 15 * time.Second})
	return &empty.Empty{}, nil
}

func (*AdbSideloadService) ConfirmEnablingAdbSideloading(ctx context.Context, request *arcpb.AdbSideloadServiceRequest) (*empty.Empty, error) {
	fullCtx := ctx
	ctx, cancel := ctxutil.Shorten(fullCtx, 10*time.Second)
	defer cancel()
	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.KeepState())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer func() error {
		if err := cr.Close(fullCtx); err != nil {
			return errors.Wrap(err, "failed to close Chrome")
		}
		return nil
	}()

	bgURL := "chrome://oobe/gaia-signin"
	tconn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
	if err != nil {
		return nil, err
	}
	defer tconn.Close()

	testing.ContextLog(ctx, "Waiting to click on the Confirm/Cancel button of the Warning UI")

	// Code variable decides which button to click and decide based on the request received by the service
	var code string
	const codeTmpl = "document.activeElement.shadowRoot.getElementById(%s).click()"
	if request.Action == "confirm" {
		code = fmt.Sprintf(codeTmpl, "'enable-adb-sideloading-ok-button'")
	} else if request.Action == "cancel" {
		code = fmt.Sprintf(codeTmpl, "'enable-adb-sideloading-cancel-button'")
	} else {
		return &empty.Empty{}, errors.Errorf("unrecognized Action: %s", request.Action)
	}

	clickErr := testing.Poll(ctx, func(ctx context.Context) error {
		if err := tconn.Eval(ctx, code, nil); err != nil {
			return errors.Wrap(err, "clicking on the Confirm/Cancel button failed")
		}
		return err
	}, &testing.PollOptions{Interval: 1 * time.Second, Timeout: 10 * time.Second})

	return &empty.Empty{}, clickErr
}
