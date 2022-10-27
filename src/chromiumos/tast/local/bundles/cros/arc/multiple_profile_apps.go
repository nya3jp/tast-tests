// Copyright 2021 The ChromiumOS Authors
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
	"chromiumos/tast/local/upstart"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         MultipleProfileApps,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Checks that ARC app from one user doesn't appear in another user",
		Contacts:     []string{"cpiao@google.com", "cros-arc-te@google.com"},
		Attr:         []string{"group:mainline", "informational", "group:arc-functional"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"ui.gaiaPoolDefault"},
		Params: []testing.Param{{
			ExtraSoftwareDeps: []string{"android_p", "chrome"},
		}, {
			Name:              "vm",
			ExtraSoftwareDeps: []string{"android_vm", "chrome"},
		}},
		Timeout: 16 * time.Minute,
	})
}

func MultipleProfileApps(ctx context.Context, s *testing.State) {
	const (
		pkgName = "org.chromium.arc.testapp.appvaliditytast"
	)
	a, err := loginAndOptin(ctx, s.RequiredVar("ui.gaiaPoolDefault"), s)
	if err != nil {
		s.Fatal("Failed to Login as First User : ", err)
	}

	if err := a.Install(ctx, arc.APKPath("ArcAppValidityTest.apk")); err != nil {
		s.Fatal("Failed installing app : ", err)
	}
	installed, err := a.PackageInstalled(ctx, pkgName)
	if err != nil {
		s.Fatal("Failed to get the installed state : ", err)
	}
	if !installed {
		s.Fatal("Failed to install app : ", err)
	}

	s.Log("Restart UI")
	if err := upstart.RestartJob(ctx, "ui"); err != nil {
		s.Fatal("Failed restarting ui : ", err)
	}

	a, err = loginAndOptin(ctx, s.RequiredVar("ui.gaiaPoolDefault"), s)
	if err != nil {
		s.Fatal("Failed to Login as Second User : ", err)
	}

	installed, err = a.PackageInstalled(ctx, pkgName)
	if err != nil {
		s.Fatal("Failed to get the installed state : ", err)
	}
	if installed {
		s.Fatal("App installed in first user appears in second user : ", err)
	}

}

func loginAndOptin(ctx context.Context, creds string, s *testing.State) (a *arc.ARC, err error) {
	cr, err := chrome.New(ctx,
		chrome.GAIALoginPool(creds),
		chrome.ARCSupported(), chrome.ExtraArgs(arc.DisableSyncFlags()...))
	if err != nil {
		return nil, errors.Wrap(err, "failed to start chrome")
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		return nil, errors.Wrap(err, "failed to create test api connection")
	}

	if err := optin.PerformAndClose(ctx, cr, tconn); err != nil {
		return nil, errors.Wrap(err, "failed to optin to play store and close")
	}

	a, err = arc.New(ctx, s.OutDir())
	if err != nil {
		return nil, errors.Wrap(err, "failed to start ARC")
	}
	return a, nil
}
