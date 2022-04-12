// Copyright 2022 The Chromium OS Authors. All rights reserved.
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

// useTconn performs an action that requires access to tconn.
func (svc *service) useTconn(ctx context.Context, fn func(tconn *chrome.TestConn) error) error {
	svc.sharedObject.ChromeMutex.Lock()
	defer svc.sharedObject.ChromeMutex.Unlock()

	cr := svc.sharedObject.Chrome
	if cr == nil {
		return errors.New("Chrome is not instantiated")
	}
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test API connection")
	}

	return fn(tconn)
}

// LaunchApp launches an app.
func (svc *service) LaunchApp(ctx context.Context, req *pb.LaunchAppRequest) (*empty.Empty, error) {
	if req.TimeoutSecs == 0 {
		req.TimeoutSecs = defaultAppLaunchTimeout
	}
	return &empty.Empty{}, svc.useTconn(ctx, func(tconn *chrome.TestConn) error {
		appID, err := getInstalledAppID(ctx, tconn, func(app *ash.ChromeApp) bool {
			return app.Name == req.AppName
		}, &testing.PollOptions{Timeout: time.Duration(req.TimeoutSecs) * time.Second})
		if err != nil {
			return err
		}
		if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.launchApp)`, appID); err != nil {
			return errors.Wrapf(err, "failed to launch app %s", req.AppName)
		}
		if err := ash.WaitForApp(ctx, tconn, appID, time.Duration(req.TimeoutSecs)*time.Second); err != nil {
			return errors.Wrapf(err, "app %s never opened", req.AppName)
		}
		return nil
	})
}

// GetPrimaryBrowser returns the App used for the primary browser.
func (svc *service) GetPrimaryBrowser(ctx context.Context, req *empty.Empty) (*pb.App, error) {
	var result *pb.App
	err := svc.useTconn(ctx, func(tconn *chrome.TestConn) error {
		app, err := PrimaryBrowser(ctx, tconn)
		if err != nil {
			return err
		}
		result = &pb.App{Id: app.ID, Name: app.Name}
		return nil
	})
	return result, err
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
