// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package loginapi

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         LaunchSharedManagedGuestSession,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test chrome.login.launchSharedManagedGuestSession Extension API",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-identity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

// LaunchSharedManagedGuestSession shares a lot of code with
// LaunchManagedGuestSession in launch_managed_guest_session.go, but
// b/204177106 is in progress, which would simplify the set up of the test.
// TODO(jityao): Refactor tests after b/204177106 is submitted.
func LaunchSharedManagedGuestSession(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepState())
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}

	defer func(ctx context.Context) {
		// Use cr as a reference to close the last started Chrome instance.
		if err := cr.Close(ctx); err != nil {
			s.Error("Failed to close Chrome connection: ", err)
		}
	}(ctx)

	// Use a shortened context for test operations to reserve time for cleanup.
	ctx, cancel := ctxutil.Shorten(ctx, 30*time.Second)
	defer cancel()

	accountID := "foo@bar.com"
	accountType := policy.AccountTypePublicSession

	policies := []policy.Policy{
		&policy.DeviceLocalAccounts{
			Val: []policy.DeviceLocalAccountInfo{
				{
					AccountID:   &accountID,
					AccountType: &accountType,
				},
			},
		},
		&policy.DeviceLoginScreenExtensions{
			Val: []string{mgs.LoginScreenExtensionID},
		},
		&policy.DeviceRestrictedManagedGuestSessionEnabled{
			Val: true,
		},
	}

	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(policies)
	pb.AddPublicAccountPolicy(accountID, &policy.ExtensionInstallForcelist{
		Val: []string{mgs.InSessionExtensionID},
	})

	if err := policyutil.ServeBlobAndRefresh(ctx, fdms, cr, pb); err != nil {
		s.Fatal("Failed to serve policies: ", err)
	}

	// Close the previous Chrome instance.
	if err := cr.Close(ctx); err != nil {
		s.Fatal("Failed to close Chrome connection: ", err)
	}

	// Restart Chrome, forcing Devtools to be available on the login screen.
	cr, err = chrome.New(ctx,
		chrome.NoLogin(),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepState(),
		chrome.ExtraArgs("--force-devtools-available"))
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

	loginScreenBGURL := chrome.ExtensionBackgroundPageURL(mgs.LoginScreenExtensionID)
	conn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(loginScreenBGURL))
	if err != nil {
		s.Fatal("Failed to connect to login screen background page: ", err)
	}
	defer conn.Close()

	// Launch a shared managed guest session.
	password := "password"
	if err := conn.Call(ctx, nil, `(password) => new Promise((resolve, reject) => {
		chrome.login.launchSharedManagedGuestSession(password, () => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		});
	})`, password); err != nil {
		s.Fatal("Failed to launch shared MGS: ", err)
	}

	select {
	case <-sw.Signals:
		// Pass
	case <-ctx.Done():
		s.Fatal("Timeout before getting SessionStateChanged signal: ", err)
	}

	inSessionBGURL := chrome.ExtensionBackgroundPageURL(mgs.InSessionExtensionID)
	inSessionConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(inSessionBGURL))
	if err != nil {
		s.Fatal("Failed to connect to in-session background page: ", err)
	}
	defer inSessionConn.Close()

	// Note that this uses lockManagedGuestSession() since locking an MGS is
	// equivalent to locking the shared session.
	if err := inSessionConn.Eval(ctx, `new Promise((resolve, reject) => {
		chrome.login.lockManagedGuestSession(() => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		});
	})`, nil); err != nil {
		s.Fatal("Failed to lock session: ", err)
	}

	// Previous conn is closed since it is a login screen extension which
	// closes when the session starts.
	conn2, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(loginScreenBGURL))
	if err != nil {
		s.Fatal("Failed to connect to login screen background page on lock screen: ", err)
	}
	defer conn2.Close()

	unlockSessionFunc := `(password) => new Promise((resolve, reject) => {
		chrome.login.unlockSharedSession(password, () => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		});
	})`

	// Attempt unlock with wrong password.
	wrongPassword := "wrong password"
	if err := conn2.Call(ctx, nil, unlockSessionFunc, wrongPassword); err == nil {
		s.Fatal("Unlock unexpectedly succeeded with wrong password")
	}

	swUnlocked, err := sm.WatchScreenIsUnlocked(ctx)
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer swUnlocked.Close(ctx)

	// Unlock with correct password.
	if err := conn2.Call(ctx, nil, unlockSessionFunc, password); err != nil {
		s.Fatal("Failed to unlock session: ", err)
	}

	select {
	case <-swUnlocked.Signals:
		// Pass
	case <-ctx.Done():
		s.Fatal("Timeout before getting session unlocked signal: ", err)
	}
}
