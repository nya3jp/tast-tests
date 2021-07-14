// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/chromeproc"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Signout,
		Desc:         "Test signout from the lock screen",
		Contacts:     []string{"chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func Signout(ctx context.Context, s *testing.State) {
	const (
		username = "testuser@gmail.com"
		password = "good"
		PIN      = "1234567890"
	)

	cr, err := chrome.New(ctx, chrome.FakeLogin(chrome.Creds{User: username, Pass: password}))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	cr.NewConn(ctx, "http://www.example.org/")
	//cr.NewConn(ctx, "chrome://hang/")
	//cr.NewConn(ctx, "chrome://crashdump/")

	// Lock the screen.
	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	oldpid, err := chromeproc.GetRootPID()
	if err != nil {
		s.Fatal("Failed to get Chrome root PID: ", err)
	}

	oldCrashes, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		s.Fatal("GetCrashes failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	ui := uiauto.New(tconn)
	if err := ui.LeftClick(nodewith.Name("Sign out").Role(role.Button))(ctx); err != nil {
		s.Log("Failed to click on Signout button: ", err)
	}
	newpid, err := chromeproc.GetRootPID()
	if err != nil {
		s.Fatal("Failed to get Chrome root PID: ", err)
	}

	if oldpid != newpid {
		s.Fatal("Chrome did not restart")
	}

	newCrashes, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		s.Fatal("GetCrashes failed: ", err)
	}

	if len(oldCrashes) != len(newCrashes) {
		s.Fatal("Chrome crashed")
	}

	if cr, err = chrome.New(ctx, chrome.NoLogin(), chrome.KeepState(), chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey"))); err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	tconn, err = cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting signing test API connection failed: ", err)
	}

	st, err := lockscreen.GetState(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get login state: ", err)
	}

	if !st.ReadyForPassword {
		s.Fatal("Chrome is not on the login screen: ", err)
	}
}
