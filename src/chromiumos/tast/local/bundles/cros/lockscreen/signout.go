// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package lockscreen

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ui/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/crash"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Signout,
		Desc:         "Test signout from the lock screen",
		Contacts:     []string{"rsorokin@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func Signout(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Getting test API connection failed: ", err)
	}

	cr.NewConn(ctx, "chrome://settings")
	cr.NewConn(ctx, "chrome://os-settings")
	cr.NewConn(ctx, "chrome://device-log")
	cr.NewConn(ctx, "chrome://flags")
	cr.NewConn(ctx, "chrome://os-credits")
	cr.NewConn(ctx, "chrome://system")

	if err := lockscreen.Lock(ctx, tconn); err != nil {
		s.Fatal("Failed to lock the screen: ", err)
	}

	oldCrashes, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		s.Fatal("GetCrashes failed: ", err)
	}

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := lockscreen.Signout(ctx, tconn); err != nil {
		s.Fatal("Failed to signout: ", err)
	}

	newCrashes, err := crash.GetCrashes(crash.DefaultDirs()...)
	if err != nil {
		s.Fatal("GetCrashes failed: ", err)
	}

	if len(oldCrashes) != len(newCrashes) {
		s.Fatal("Chrome crashed during the test")
	}

	// Restart chrome for testing
	if cr, err = chrome.New(ctx, chrome.NoLogin(), chrome.KeepState(), chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey"))); err != nil {
		s.Fatal("Chrome restart for testing failed: ", err)
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
