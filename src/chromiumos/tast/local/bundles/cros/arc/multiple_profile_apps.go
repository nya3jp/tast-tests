// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package arc

import (
	"context"
	"time"

	"chromiumos/tast/errors"
	"chromiumos/tast/local/arc"
	"chromiumos/tast/local/arc/optin"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultipleProfileApps,
		Desc:         "Checks that ARC app from one user doesn't appear in another user",
		Contacts:     []string{"rnanjappan@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 2*chrome.GAIALoginTimeout + 2*time.Minute,
	})
}

func MultipleProfileApps(ctx context.Context, s *testing.State) {
	if err := loginAsFirstUserAndInstallApp(ctx, s.RequiredVar("ui.gaiaPoolDefault"), s); err != nil {
		s.Fatal("Failed to Login as First User and Install App : ", err)
	}
	if err := loginAsSecondUserAndVerifyApp(ctx, s.RequiredVar("ui.gaiaPoolDefault"), s); err != nil {
		s.Fatal("Failed to Login as Second User and Verify App : ", err)
	}
}

func loginAsFirstUserAndInstallApp(ctx context.Context, creds string, s *testing.State) (err error) {
	testing.ContextLog(ctx, "Login as first user")
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(creds),
		chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		return errors.Wrap(err, "failed to start chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test api connection")
	}

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to optin to play store and close")
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		return errors.Wrap(err, "failed to start ARC")
	}

	if err := a.Install(ctx, arc.APKPath("ArcAppValidityTest.apk")); err != nil {
		return errors.Wrap(err, "failed to install app")
		s.Fatal("Failed installing app: ", err)
	}

	act, err := arc.NewActivity(a, "org.chromium.arc.testapp.appvaliditytast", ".MainActivity")
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err != nil {
		return errors.Wrap(err, "failed to start activity")
	}

	testing.ContextLog(ctx, "Restart UI")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		return errors.Wrap(err, "failed to restart ui")
	}
	return nil
}

func loginAsSecondUserAndVerifyApp(ctx context.Context, creds string, s *testing.State) (err error) {
	testing.ContextLog(ctx, "Login as second user")
	// chrome.KeepState() is needed to show the login
	// screen with a user (instead of the OOBE login screen).
	cr, err := chrome.New(
		ctx,
		chrome.GAIALoginPool(creds),
		chrome.KeepState(),
		chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...),
	)
	if err != nil {
		return errors.Wrap(err, "failed to start chrome")
	}
	defer cr.Close(ctx)

	// Open the test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to create test api connection")
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		return errors.Wrap(err, "failed to optin to play store and close")
	}

	a, err := arc.New(ctx, s.OutDir())
	if err != nil {
		return errors.Wrap(err, "failed to start ARC")
	}
	defer a.Close(ctx)

	act, err := arc.NewActivity(a, "org.chromium.arc.testapp.appvaliditytast", ".MainActivity")
	if err != nil {
		return errors.Wrap(err, "failed to create new activity")
	}
	defer act.Close()

	if err := act.Start(ctx, tconn); err == nil {
		return errors.Wrap(err, "app installed by first user appears in second user")
	}
	return nil
}
