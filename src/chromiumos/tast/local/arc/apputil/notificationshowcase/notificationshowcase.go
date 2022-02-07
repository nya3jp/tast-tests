// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

// Package notificationshowcase is used for writing test cases which use Notification Showcase app.
package notificationshowcase

import (
	"context"

	"chromiumos/tast/common/android/adb"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/apputil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/input"
)

const (
	// AppName is the name of Notification Showcase app.
	AppName = "Notification Showcase"
	// PkgName is the package name of Notification Showcase app.
	PkgName = "org.chromium.arc.testapp.notification2"
	// AppID is the app ID of Notification Showcase app.
	AppID = "ealboieclppkdlimieennaeddhpemdmo"
)

// NotificationShowcase represents an instance of the Notification Showcase app.
type NotificationShowcase struct {
	*apputil.App
	sdkPath string
}

// NewApp creates a new instance of the Android application version of gmail.
// This app requires the actual apk file in order to install.
// App installation and launch are needed after initialization.
func NewApp(ctx context.Context, a *arc.ARC, tconn *chrome.TestConn, kb *input.KeyboardEventWriter, sdkPath string) (*NotificationShowcase, error) {
	app, err := apputil.NewApp(ctx, kb, tconn, a, AppName, PkgName)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create Notification Showcase app instance")
	}

	return &NotificationShowcase{app, sdkPath}, nil
}

// Install installs the Notification Showcase app.
func (app *NotificationShowcase) Install(ctx context.Context) error {
	return app.ARC.Install(ctx, app.sdkPath, adb.InstallOptionGrantPermissions)
}
