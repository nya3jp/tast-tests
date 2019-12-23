// Copyright 2020 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	//"bufio"
	//"io/ioutil"
	//"os"
	//"regexp"
	//"strconv"
	"fmt"
	"time"

	//"github.com/golang/protobuf/ptypes"
	"github.com/golang/protobuf/ptypes/empty"
	"golang.org/x/net/context"
	"google.golang.org/grpc"

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

// AdbSideloadService implements tast.cros.arc.AdbSideloadService.
type AdbSideloadService struct {
	s *testing.ServiceState
}

func (*AdbSideloadService) EnableAdbSideloadFlag(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	//cr, err := chrome.New(ctx)

	cr, err := chrome.New(ctx, chrome.NoLogin(), chrome.KeepState(), chrome.ExtraArgs("--load-extension=/usr/local/autotest/common_lib/cros/autotest_private_ext", "--disable-extensions-except=/usr/local/autotest/common_lib/cros/autotest_private_ext"))

	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "Creating test API connection failed")
	}
	defer tconn.Close()

	/*
		code := fmt.Sprintf("tast.promisify(chrome.autotestPrivate.setWhitelistedPref)('EnableAdbSideloadingRequested', true)")
		if err := tconn.EvalPromise(ctx, code, nil); err != nil {
			return nil, errors.Wrap(err, "Setting Prefs failed2")
		}
	*/
	/*
		if err := tconn.EvalPromise(ctx, `new Promise((resolve, reject) => {
			chrome.autotestPrivate.setWhitelistedPref('EnableAdbSideloadingRequested', true, () => {
					if (chrome.runtime.lastError) {reject(chrome.runtime.lastError.message);
					else {
						resolve();
					};
				}
			);
		})`, nil); err != nil {
			return &empty.Empty{}, nil
		}
	*/

	if err := tconn.Exec(ctx, `
	new Promise((resolve, reject) => {
			chrome.autotestPrivate.setWhitelistedPref('EnableAdbSideloadingRequested', true, () => {
						resolve();
							});
						})`); err != nil {
		testing.ContextLog(ctx, "Error Received while setting the Sideloading Flag ")

		//s.Fatal("Failed to set listener for 'copy' event: ", err)
	}

	// VAibhav

	//if err := upstart.RestartJob(ctx, "ui"); err != nil {
	//		return nil, errors.Wrap(err, "Setting Prefs failed2")
	//}

	/*

		code = fmt.Sprintf("document.activeElement.shadowRoot.getElementById('enable-adb-sideloading-ok-button').shadowRoot.getElementById('textButton').click()")
		if err := tconn.EvalPromise(ctx, code, nil); err != nil {
			return nil, errors.Wrap(err, "Clicking on the OK button failed")
		}
	*/
	return &empty.Empty{}, nil
}

func (*AdbSideloadService) EnableAdbConfirm(ctx context.Context, req *empty.Empty) (*empty.Empty, error) {

	cr, err := chrome.New(ctx, chrome.ARCDisabled(), chrome.NoLogin(), chrome.KeepState())
	if err != nil {
		return nil, errors.Wrap(err, "failed to connect to Chrome")
	}
	defer cr.Close(ctx)

	//tconn, err := cr.TestAPIConn(ctx)
	//if err != nil {
	//	return nil, errors.Wrap(err, "Creating test API connection failed")
	//}
	//defer tconn.Close()

	bgURL := chrome.ExtensionBackgroundPageURL("chrome://oobe/gaia-signin")
	testing.ContextLog(ctx, "BG Url = ", bgURL)
	tconn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL("chrome://oobe/gaia-signin"))
	if err != nil {
		return nil, err
	}
	defer tconn.Close()

	testing.ContextLog(ctx, "Tconn = ", tconn)

	time.Sleep(10 * time.Second)
	code := fmt.Sprintf("document.activeElement.shadowRoot.getElementById('enable-adb-sideloading-ok-button').shadowRoot.getElementById('textButton').click()")
	if err := tconn.EvalPromise(ctx, code, nil); err != nil {
		return nil, errors.Wrap(err, "Clicking on the OK button failed")
	}

	return &empty.Empty{}, nil
}
