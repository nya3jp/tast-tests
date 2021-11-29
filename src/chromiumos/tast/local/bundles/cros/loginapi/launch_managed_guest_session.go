// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package loginapi

import (
	"context"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchManagedGuestSession,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Test chrome.login.launchManagedGuestSession Extension API",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-identity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

func LaunchManagedGuestSession(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	accountID := "foo@bar.com"
	// These extensions are unlisted on the Chrome Web Store but can be
	// downloaded directly using the extension IDs.
	// The code for the extensions can be found in the Chromium repo at
	// chrome/test/data/extensions/api_test/login_screen_apis/.
	// ID for "Login screen APIs test extension".
	loginScreenExtensionID := "oclffehlkdgibkainkilopaalpdobkan"
	// ID for "Login screen APIs in-session test extension".
	inSessionExtensionID := "ofcpkomnogjenhfajfjadjmjppbegnad"

	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.Accounts(accountID),
		mgs.AddPublicAccountPolicies(accountID, []policy.Policy{
			&policy.ExtensionInstallForcelist{Val: []string{inSessionExtensionID}},
		}),
		mgs.ExtraPolicies([]policy.Policy{
			&policy.DeviceLoginScreenExtensions{Val: []string{loginScreenExtensionID}},
		}),
		mgs.ExtraChromeOptions(
			chrome.ExtraArgs("--force-devtools-available"),
		),
	)
	if err != nil {
		s.Fatal("Failed to start Chrome on Signin screen with MGS accounts: ", err)
	}
	defer func() {
		if err := mgs.Close(ctx); err != nil {
			s.Fatal("Failed close MGS: ", err)
		}
	}()

	sm, err := session.NewSessionManager(ctx)
	if err != nil {
		s.Fatal("Failed to connect to session manager: ", err)
	}

	sw, err := sm.WatchSessionStateChanged(ctx, "started")
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer sw.Close(ctx)

	loginScreenBGURL := chrome.ExtensionBackgroundPageURL(loginScreenExtensionID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(loginScreenBGURL))
	if err != nil {
		s.Fatal("Failed to connect to login screen background page: ", err)
	}
	defer conn.Close()

	if err := conn.Eval(ctx, `new Promise((resolve, reject) => {
		chrome.login.launchManagedGuestSession(() => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		});
	})`, nil); err != nil {
		s.Fatal("Failed to launch MGS: ", err)
	}

	select {
	case <-sw.Signals:
		// Pass
	case <-ctx.Done():
		s.Fatal("Timeout before getting SessionStateChanged signal: ", err)
	}

	inSessionBGURL := chrome.ExtensionBackgroundPageURL(inSessionExtensionID)
	inSessionConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(inSessionBGURL))
	if err != nil {
		s.Fatal("Failed to connect to in-session background page: ", err)
	}
	defer inSessionConn.Close()
}
