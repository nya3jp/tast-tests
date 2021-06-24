// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DeviceGuestModeEnabled,
		Desc: "Test chrome.login.launchManagedGuestSession Extension API",
		Contacts: []string{
			"vsavu@google.com", // Test author
			"chromeos-commercial-stability@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
		VarDeps:      []string{"ui.signinProfileTestExtensionManifestKey"},
	})
}

func DeviceGuestModeEnabled(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.NoLogin(),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment(),
		chrome.ExtraArgs("--disable-policy-key-verification"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer func(ctx context.Context) {
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Creating login test API connection failed: ", err)
	}
	defer tconn.Close()

	uia := uiauto.New(tconn)

	for _, param := range []struct {
		name             string
		guestModeAllowed bool // guestModeAllowed indicates whether it is possible to log in as a guest.
		value            policy.Policy
	}{
		{
			name:             "unset",
			guestModeAllowed: true,
			value:            &policy.DeviceGuestModeEnabled{Stat: policy.StatusUnset},
		},
		{
			name:             "enabled",
			guestModeAllowed: true,
			value:            &policy.DeviceGuestModeEnabled{Val: true},
		},
		{
			name:             "disabled",
			guestModeAllowed: false,
			value:            &policy.DeviceGuestModeEnabled{Val: false},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeOnErrorToFile(ctx, s.OutDir(), s.HasError, tconn, "ui_tree_"+param.name)

			pb := fakedms.NewPolicyBlob()
			pb.AddPolicy(param.value)

			if err := fdms.WritePolicyBlob(pb); err != nil {
				s.Fatal("Failed to set policies: ", err)
			}

			if err := policyutil.Refresh(ctx, tconn); err != nil {
				s.Fatal("Failed to refresh policies: ", err)
			}

			gmNode := nodewith.Name("Browse as Guest").First()
			if param.guestModeAllowed {
				if err := uia.WaitUntilExists(gmNode)(ctx); err != nil {
					s.Error("Guest mode button did not appear: ", err)
				}
			} else {
				// TODO(b/190596080): Change to aui.EnsureGoreFor once the
				// problem is resolved.
				// Confirm that the virtual keyboard does not exist.
				if err := uia.WaitUntilExists(gmNode)(ctx); err == nil {
					s.Error("Guest mode button appeared")
				}
			}
		})
	}
}
