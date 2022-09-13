// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package apps

import (
	"context"
	"time"

	"github.com/golang/protobuf/ptypes/empty"
	"google.golang.org/grpc"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/common"
	pb "chromiumos/tast/services/cros/apps"
	"chromiumos/tast/testing"
)

const defaultAppLaunchTimeout = 60

func init() {
	var svc service
	testing.AddService(&testing.Service{
		Register: func(srv *grpc.Server, s *testing.ServiceState) {
			svc = service{sharedObject: common.SharedObjectsForServiceSingleton}
			pb.RegisterAppsServiceServer(srv, &svc)
		},
		GuaranteeCompatibility: true,
	})
}

// Service implements tast.cros.apps.AppsService.
type service struct {
	sharedObject *common.SharedObjectsForService
}

// LaunchApp launches an app.
func (svc *service) LaunchApp(ctx context.Context, req *pb.LaunchAppRequest) (*empty.Empty, error) {
	if req.TimeoutSecs == 0 {
		req.TimeoutSecs = defaultAppLaunchTimeout
	}
	return common.UseTconn(ctx, svc.sharedObject, func(tconn *chrome.TestConn) (*empty.Empty, error) {
		appID, err := getInstalledAppID(ctx, tconn, func(app *ash.ChromeApp) bool {
			return app.Name == req.AppName
		}, &testing.PollOptions{Timeout: time.Duration(req.TimeoutSecs) * time.Second})
		if err != nil {
			return nil, err
		}
		if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.launchApp)`, appID); err != nil {
			return nil, errors.Wrapf(err, "failed to launch app %s", req.AppName)
		}
		if err := ash.WaitForApp(ctx, tconn, appID, time.Duration(req.TimeoutSecs)*time.Second); err != nil {
			return nil, errors.Wrapf(err, "app %s never opened", req.AppName)
		}
		return &empty.Empty{}, nil
	})
}

// GetPrimaryBrowser returns the App used for the primary browser.
func (svc *service) GetPrimaryBrowser(ctx context.Context, req *empty.Empty) (*pb.App, error) {
	return common.UseTconn(ctx, svc.sharedObject, func(tconn *chrome.TestConn) (*pb.App, error) {
		app, err := PrimaryBrowser(ctx, tconn)
		if err != nil {
			return nil, err
		}
		return &pb.App{Id: app.ID, Name: app.Name}, nil
	})
}

// LaunchPrimaryBrowser launches the primary browser and returns the App launched.
func (svc *service) LaunchPrimaryBrowser(ctx context.Context, req *empty.Empty) (*pb.App, error) {
	app, err := svc.GetPrimaryBrowser(ctx, req)
	if err != nil {
		return app, err
	}
	_, err = svc.LaunchApp(ctx, &pb.LaunchAppRequest{AppName: app.Name})
	return app, err
}
