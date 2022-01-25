// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DeviceShowUserNamesOnSignin,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test the DeviceShowUserNamesOnSignin policy",
		Contacts: []string{
			"rsorokin@google.com", // Test author
			"cros-oac@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
		Timeout:      chrome.LoginTimeout + 40*time.Second,
	})
}

func DeviceShowUserNamesOnSignin(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	cleanUpCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx, chrome.KeepEnrollment(), chrome.DMSPolicy(fdms.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	username := cr.Creds().User
	cr.Close(ctx)

	// Start a new Chrome instance with the login screen.
	cr, err = chrome.New(ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
		chrome.KeepState())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer cr.Close(cleanUpCtx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}

	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)

	for _, param := range []struct {
		name          string
		showUserNames bool
		value         policy.Policy
	}{
		{
			name:          "unset",
			showUserNames: true,
			value:         &policy.DeviceShowUserNamesOnSignin{Stat: policy.StatusUnset},
		},
		{
			name:          "enabled",
			showUserNames: true,
			value:         &policy.DeviceShowUserNamesOnSignin{Val: true},
		},
		{
			name:          "disabled",
			showUserNames: false,
			value:         &policy.DeviceShowUserNamesOnSignin{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(cleanupCtx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name)
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, []policy.Policy{param.value}); err != nil {
				s.Fatal("Failed to serve and verify policies: ", err)
			}

			if param.showUserNames {
				podName := nodewith.Name(username).Role(role.Button).Ancestor(nodewith.ClassName("UserView"))
				if err := ui.WaitUntilExists(podName)(ctx); err != nil {
					s.Error("Userpod did not appear: ", err)
				}
			} else {
				webviewName := nodewith.Role(role.Iframe).Ancestor(nodewith.ClassName("OobeWebDialogView"))
				if err := ui.WaitUntilExists(webviewName)(ctx); err != nil {
					s.Error("Gaia login did not appear: ", err)
				}
			}
		})
	}
}
