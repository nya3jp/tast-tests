// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: ChromeLoginAPI,
		Desc: "Behavior of the chrome.login Extension API",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-identity@google.com",
		},
		Attr:         []string{"group:enrollment"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func ChromeLoginAPI(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepState(),
		chrome.ExtraArgs("--disable-policy-key-verification"))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	s.Run(ctx, "Launch managed guest session", func(ctx context.Context, s *testing.State) {
		accountID := "foo@bar.com"
		accountType := policy.ACCOUNT_TYPE_PUBLIC_SESSION
		loginScreenExtensionID := "oclffehlkdgibkainkilopaalpdobkan"
		inSessionExtensionID := "ofcpkomnogjenhfajfjadjmjppbegnad"

		policies := []policy.Policy{
			&policy.DeviceLocalAccounts{
				Val: []policy.DeviceLocalAccountInfo{
					{
						AccountId:   &accountID,
						AccountType: &accountType,
					},
				},
			},
			&policy.DeviceLoginScreenExtensions{
				Val: []string{loginScreenExtensionID},
			},
			&policy.PublicAccountPolicy{
				Policy: &policy.ExtensionInstallForcelist{
					Val: []string{inSessionExtensionID},
				},
			},
		}

		// Update policies.
		if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
			s.Fatal("Failed to update policies: ", err)
		}

		// Close the previous Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}

		// Restart Chrome.
		cr, err = chrome.New(ctx,
			chrome.NoLogin(),
			chrome.DMSPolicy(fdms.URL),
			chrome.KeepState(),
			chrome.ExtraArgs("--disable-policy-key-verification"))
		if err != nil {
			s.Fatal("Chrome restart failed: ", err)
		}

		sm, err := session.NewSessionManager(ctx)
		if err != nil {
			s.Fatal("Failed to connect to session manager: ", err)
		}

		sw, err := sm.WatchSessionStateChanged(ctx, "started")
		if err != nil {
			s.Fatal("Failed to watch for D-Bus signals: ", err)
		}
		defer sw.Close(ctx)

		bgURL := chrome.ExtensionBackgroundPageURL(loginScreenExtensionID)
		conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(bgURL))
		if err != nil {
			s.Fatal("Failed to connect to background page: ", err)
		}
		defer conn.Close()

		if err := conn.EvalPromise(ctx,
			`new Promise((resolve, reject) => {
			chrome.login.launchManagedGuestSession(() => {
				if (chrome.runtime.lastError) {
					reject(new Error(chrome.runtime.lastError.message));
				}
				resolve();
			});
		})`, nil); err != nil {
			s.Fatal("Failed to launch MGS: ", err)
		}

		select {
		case <-sw.Signals:
			return
		case <-ctx.Done():
			s.Fatal("Timeout before getting SessionStateChanged signal: ", err)
		}
	})
}
