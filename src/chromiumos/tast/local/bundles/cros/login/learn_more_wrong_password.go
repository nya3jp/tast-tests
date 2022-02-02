// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package login

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/lockscreen"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LearnMoreWrongPassword,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Tests that there is a \"Learn More\" button that shows help dialog if user enters wrong password twice",
		Contacts:     []string{"bohdanty@google.com", "chromeos-sw-engprod@google.com", "cros-oac@google.com"},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      chrome.LoginTimeout + 30*time.Second,
	})
}

func LearnMoreWrongPassword(ctx context.Context, s *testing.State) {
	// Create user on the device.
	cr, err := chrome.New(ctx)
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	creds := cr.Creds()
	cr.Close(ctx)

	cr, err = chrome.New(ctx,
		chrome.ExtraArgs("--skip-force-online-signin-for-testing"),
		chrome.NoLogin(),
		chrome.KeepState(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")))

	if err != nil {
		s.Fatal("Chrome start failed: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)

	if err != nil {
		s.Fatal("Getting test Signin Profile API connection failed: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	if err := lockscreen.WaitForPasswordField(ctx, tconn, creds.User, 10*time.Second); err != nil {
		s.Fatal("Failed to wait for the password field: ", err)
	}
	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get virtual keyboard: ", err)
	}
	defer keyboard.Close()

	// Enter wrong password
	if err := lockscreen.EnterPassword(ctx, tconn, creds.User, creds.Pass+"fake", keyboard); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}

	ui := uiauto.New(tconn)
	if err := ui.WaitUntilExists(lockscreen.AuthErrorFinder)(ctx); err != nil {
		s.Fatal("Failed to wait for auth error: ", err)
	}

	// Enter wrong password again
	if err := lockscreen.EnterPassword(ctx, tconn, creds.User, creds.Pass+"fake", keyboard); err != nil {
		s.Fatal("Failed to enter password: ", err)
	}

	if err := ui.WaitUntilExists(lockscreen.ConsecutiveAuthErrorFinder)(ctx); err != nil {
		s.Fatal("Failed to wait for consecutive auth error: ", err)
	}

	if err := ui.LeftClick(nodewith.Role(role.Button).NameStartingWith("Learn more"))(ctx); err != nil {
		s.Fatal("Failed to click Learn more button")
	}

	if err := ui.WaitUntilExists(nodewith.Role(role.Window).NameStartingWith("Help"))(ctx); err != nil {
		s.Fatal("Failed to wait for help diaglog")
	}

	if err := ui.LeftClick(nodewith.Role(role.Button).NameStartingWith("Close"))(ctx); err != nil {
		s.Fatal("Failed to close help dialog")
	}

	if err := ui.WaitUntilGone(nodewith.Role(role.Window).NameStartingWith("Help"))(ctx); err != nil {
		s.Fatal("Failed to wait for help dialog to close")
	}
}
