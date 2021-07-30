// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"fmt"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: LoginScreenUIAPI,
		Desc: "Test chrome.loginScreenUi Extension API",
		Contacts: []string{
			"jityao@google.com", // Test author
			"chromeos-commercial-identity@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "fakeDMSEnrolled",
	})
}

func LoginScreenUIAPI(ctx context.Context, s *testing.State) {
	fdms := s.FixtValue().(*fakedms.FakeDMS)

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fdms.URL),
		chrome.KeepEnrollment())
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

	// This extension is unlisted on the Chrome Web Store but can be
	// downloaded directly using the extension IDs.
	// ID for "Login screen APIs test extension".
	// The code for the extension can be found in the Chromium repo at
	// chrome/test/data/extensions/api_test/login_screen_apis/extension.
	loginScreenExtensionID := "oclffehlkdgibkainkilopaalpdobkan"

	policies := []policy.Policy{
		&policy.DeviceLoginScreenExtensions{
			Val: []string{loginScreenExtensionID},
		},
	}

	if err := policyutil.ServeAndRefresh(ctx, fdms, cr, policies); err != nil {
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
		chrome.KeepEnrollment(),
		chrome.ExtraArgs("--force-devtools-available"))
	if err != nil {
		s.Fatal("Chrome restart failed: ", err)
	}

	loginScreenBGURL := chrome.ExtensionBackgroundPageURL(loginScreenExtensionID)
	bgConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(loginScreenBGURL))
	if err != nil {
		s.Fatal("Failed to connect to login screen background page: ", err)
	}
	defer bgConn.Close()

	// Show window.html.
	// The file window.html is bundled with the extension.
	if err := bgConn.EvalPromiseDeprecated(ctx,
		`new Promise((resolve, reject) => {
		chrome.loginScreenUi.show({url: "window.html"}, () => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
			}
			resolve();
		});
	})`, nil); err != nil {
		s.Fatal("Failed to show window: ", err)
	}

	windowURL := fmt.Sprintf("chrome-extension://%s/window.html", loginScreenExtensionID)
	windowConn, err := cr.NewConnForTarget(ctx, chrome.MatchTargetURL(windowURL))
	if err != nil {
		s.Fatal("Failed to connect to window: ", err)
	}
	defer windowConn.Close()

	windowCtx, cancel := context.WithTimeout(ctx, 1*time.Second)
	defer cancel()

	// Check that the window title is correct.
	// WaitForExpr has to be used since the window title is not updated immediately.
	expectedWindowTitle := "Login screen APIs test extension"
	expr := fmt.Sprintf(`document.querySelector('title').innerText === '%s'`, expectedWindowTitle)
	if err := windowConn.WaitForExpr(windowCtx, expr); err != nil {
		s.Error("Window title does not match: ", err)
	}

	// Close the window.
	if err := bgConn.EvalPromiseDeprecated(ctx,
		`new Promise((resolve, reject) => {
		chrome.loginScreenUi.close(() => {
			if (chrome.runtime.lastError) {
				reject(new Error(chrome.runtime.lastError.message));
			}
			resolve();
		});
	})`, nil); err != nil {
		s.Fatal("Failed to close window: ", err)
	}

	// Check that the window is closed.
	available, err := cr.IsTargetAvailable(ctx, chrome.MatchTargetURL(windowURL))
	if err != nil {
		s.Fatal("Failed to get targets: ", err)
	}
	if available {
		s.Error("Window was not closed")
	}
}
