// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/mgs"
	"chromiumos/tast/local/session"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SharedManagedGuestSessionCleanup,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test chrome.login.endSharedSession Extension API properly performs cleanup",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-identity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      fixture.FakeDMSEnrolled,
	})
}

// SharedManagedGuestSessionCleanup tests that chrome.login.endSession performs
// its cleanup operations correctly. The following cleanups are tested:
// 1. Browsing data: This is tested by opening a browser page, setting a
//    cookie, and checking that both the browser history and cookie are cleared
//    after cleanup.
// 2. Open windows: This is tested by opening a browser tab and checking that
//    the tab is closed.
// 3. Extensions: This is tested by checking that the background page
//   connection is closed after cleanup. This is not a direct check since we
//   cannot test if an extension has been reinstalled.
//   The RestrictedManagedGuestSessionExtensionCleanupExemptList policy is also
//   tested here.
// 4. Clipboard: This is tested by setting clipboard data and checking that it
//    is cleared.
// Printing is not tested due to the set up needed and will be covered in a
// browser test in Chrome instead.
func SharedManagedGuestSessionCleanup(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	accountID := "foo@bar.com"

	loginScreenExtensionID := mgs.LoginScreenExtensionID
	inSessionExtensionID := mgs.InSessionExtensionID

	// ID for Google Keep extension. Note this extension is arbitrarily chosen
	// and is used to test the
	// RestrictedManagedGuestSessionExtensionCleanupExemptList policy.
	googleKeepExtensionID := "lpcaedmchfhocbbapmcbpinfpgnhiddi"
	// ID for the Test API extension.
	testAPIExtensionID := "behllobkkfkfnphdnhnkndlbkcpglgmj"

	mgs, cr, err := mgs.New(
		ctx,
		fdms,
		mgs.Accounts(accountID),
		mgs.AddPublicAccountPolicies(accountID, []policy.Policy{
			&policy.ExtensionInstallForcelist{
				Val: []string{inSessionExtensionID, googleKeepExtensionID},
			},
			&policy.RestrictedManagedGuestSessionExtensionCleanupExemptList{
				Val: []string{inSessionExtensionID, testAPIExtensionID},
			},
		}),
		mgs.ExtraPolicies([]policy.Policy{
			&policy.DeviceLoginScreenExtensions{
				Val: []string{loginScreenExtensionID},
			},
			&policy.DeviceRestrictedManagedGuestSessionEnabled{
				Val: true,
			},
		}),
		mgs.ExtraChromeOptions(chrome.ExtraArgs("--force-devtools-available")),
	)
	if err != nil {
		s.Error("Failed to start Chrome on Signin screen with MGS accounts: ", err)
	}
	defer mgs.Close(ctx)

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

	inSessionBGURL := chrome.ExtensionBackgroundPageURL(inSessionExtensionID)
	inSessionConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(inSessionBGURL))
	if err != nil {
		s.Fatal("Failed to connect to in-session background page: ", err)
	}
	defer inSessionConn.Close()

	swLocked, err := sm.WatchScreenIsLocked(ctx)
	if err != nil {
		s.Fatal("Failed watch for screen lock: ", err)
	}

	googleKeepBGURL := chrome.ExtensionBackgroundPageURL(googleKeepExtensionID)
	googleKeepConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(googleKeepBGURL))
	if err != nil {
		s.Fatal("Failed to connect to Google Keep background page: ", err)
	}
	defer googleKeepConn.Close()

	// Store arbitrary data in localStorage of the Google Keep extension.
	if err := googleKeepConn.Eval(ctx, `new Promise((resolve, reject) => {
		chrome.storage.local.set({foo: 1}, () => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		});
	})`, nil); err != nil {
		s.Fatal("Failed to set localStorage for Google Keep: ", err)
	}

	pageConn, err := cr.NewConn(ctx, "https://www.example.com")
	if err != nil {
		s.Fatal("Failed to open www.example.com: ", err)
	}
	defer pageConn.Close()

	// Set the cookie for www.example.com. www.example.com itself does not set
	// any cookies, so this is safe.
	if err := pageConn.Eval(ctx, "document.cookie = 'abcdef'", nil); err != nil {
		s.Fatal("Failed to set cookie: ", err)
	}

	tConn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	// Set clipboard data.
	if err := ash.SetClipboard(ctx, tConn, "clipboard string"); err != nil {
		s.Fatal("Failed to set clipboard: ", err)
	}

	// Call login.endSharedSession() to end the shared session and trigger
	// cleanup. At the end of the cleanup, the screen will be locked.
	if err := inSessionConn.Eval(ctx, `new Promise((resolve, reject) => {
		chrome.login.endSharedSession(() => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		});
	})`, nil); err != nil {
		s.Fatal("Failed to end shared session: ", err)
	}

	select {
	case <-swLocked.Signals:
		// Pass
	case <-ctx.Done():
		s.Fatal("Timeout before getting session locked signal: ", err)
	}

	swUnlocked, err := sm.WatchScreenIsUnlocked(ctx)
	if err != nil {
		s.Fatal("Failed to watch for D-Bus signals: ", err)
	}
	defer swUnlocked.Close(ctx)

	// Previous conn is closed since it is a login screen extension which
	// closes when the session starts.
	conn2, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(loginScreenBGURL))
	if err != nil {
		s.Fatal("Failed to connect to login screen background page on lock screen: ", err)
	}
	defer conn2.Close()

	// Enter a new shared session.
	if err := conn2.Call(ctx, nil, `(password) => new Promise((resolve, reject) => {
		chrome.login.enterSharedSession(password, () => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			resolve();
		});
	})`, password); err != nil {
		s.Fatal("Failed to enter new shared session: ", err)
	}

	select {
	case <-swUnlocked.Signals:
		// Pass
	case <-ctx.Done():
		s.Fatal("Timeout before getting session unlocked signal: ", err)
	}

	// Check the inSessionConn is still alive. This indicates that the
	// RestrictedManagedGuestSessionExtensionCleanupExemptList was successfully
	// applied.
	if err := checkConnIsAlive(ctx, inSessionConn); err != nil {
		s.Fatal("In-session extension conn closed unexpectedly: ", err)
	}

	// Cleanup should have closed the Google Keep extension connection.
	if err := checkConnIsAlive(ctx, googleKeepConn); err == nil {
		s.Fatal("Google Keep extension conn was not closed: ", err)
	}

	// Create new connection for Google Keep since the old one was closed.
	googleKeepConn2, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(googleKeepBGURL))
	if err != nil {
		s.Fatal("Failed to connect to Google Keep background page: ", err)
	}
	defer googleKeepConn2.Close()

	// Check that localStorage is cleared.
	if err := googleKeepConn2.Eval(ctx, `new Promise((resolve, reject) => {
		chrome.storage.local.get((data) => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
				return;
			}
			if (typeof data !== 'object'  || Object.keys(data).length !== 0) {
				reject(new Error("Expected {}, got: " + JSON.stringify(data)));
				return;
			}
			resolve();
		});
	})`, nil); err != nil {
		s.Fatal("Local storage for Google Keep was not cleared: ", err)
	}

	// Cleanup should have closed all open browser windows.
	if err := pageConn.Eval(ctx, "undefined", nil); err == nil {
		s.Fatal("Page conn was not closed: ", err)
	}

	// Open the browsing history page.
	historyConn, err := cr.NewConn(ctx, "chrome://history")
	if err != nil {
		s.Fatal("Failed to open chrome://history: ", err)
	}
	defer historyConn.Close()

	ui := uiauto.New(tConn)

	// Check that there are no history entries. EnsureGoneFor is needed as the
	// UI tree is not immediately populated so the node will not be present
	// initially.
	if err := ui.EnsureGoneFor(nodewith.HasClass("website-link").Role(role.Link), 5*time.Second)(ctx); err != nil {
		s.Fatal("Browser history was not cleared: ", err)
	}

	clipboardText, err := ash.ClipboardTextData(ctx, tConn)
	if err != nil {
		s.Fatal("Failed to get clipboard text: ", err)
	}
	if clipboardText != "" {
		s.Fatal("Clipboard was not cleared: ", clipboardText)
	}
}

func checkConnIsAlive(ctx context.Context, conn *chrome.Conn) error {
	result := false
	if err := conn.Eval(ctx, "true", &result); err != nil {
		return err
	}
	if !result {
		return errors.New("eval 'true' returned false")
	}
	return nil
}
