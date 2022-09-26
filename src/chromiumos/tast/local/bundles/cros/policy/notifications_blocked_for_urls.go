// Copyright 2020 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/pci"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         NotificationsBlockedForUrls,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Behavior of NotificationsBlockedForUrls policy: checking if notifications are blocked for a specified url",
		Contacts: []string{
			"swapnilgupta@google.com", // Test author
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Params: []testing.Param{{
			Fixture: fixture.ChromePolicyLoggedIn,
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			ExtraAttr:         []string{"informational"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val:               browser.TypeLacros,
		}},
		Data: []string{"notifications_for_urls_test_page.html"},
		SearchFlags: []*testing.StringPair{
			pci.SearchFlag(&policy.NotificationsBlockedForUrls{}, pci.VerifiedFunctionalityJS),
			pci.SearchFlag(&policy.DefaultNotificationsSetting{}, pci.VerifiedValue),
		},
	})
}

// NotificationsBlockedForUrls tests the NotificationsBlockedForUrls policy.
func NotificationsBlockedForUrls(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fdms := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Reserve ten seconds for cleanup.
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	includesBlockedURL := []string{"http://www.bing.com", server.URL,
		"https://www.yahoo.com"}
	excludesBlockedURL := []string{"http://www.bing.com",
		"https://www.irs.gov/",
		"https://www.yahoo.com"}

	for _, param := range []struct {
		name                string
		notificationBlocked bool            // notification_blocked determines whether notifications are blocked in the test case or not.
		policies            []policy.Policy // list of policies to be set.
	}{
		{
			name:                "site_blocked_block",
			notificationBlocked: true,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Val: includesBlockedURL},
				&policy.DefaultNotificationsSetting{Val: 1},
			},
		},
		{
			name:                "site_allowed_show",
			notificationBlocked: false,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Val: excludesBlockedURL},
				&policy.DefaultNotificationsSetting{Val: 1},
			},
		},
		{
			name:                "not_set_show",
			notificationBlocked: false,
			policies: []policy.Policy{
				&policy.NotificationsBlockedForUrls{Stat: policy.StatusUnset},
				&policy.DefaultNotificationsSetting{Val: 1},
			},
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			// Perform cleanup.
			if err := policyutil.ResetChrome(ctx, fdms, cr); err != nil {
				s.Fatal("Failed to clean up: ", err)
			}

			// Update policies.
			if err := policyutil.ServeAndVerify(ctx, fdms, cr, param.policies); err != nil {
				s.Fatal("Failed to update policies: ", err)
			}

			// TODO(crbug.com/1259615): This should be part of the fixture.
			br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
			if err != nil {
				s.Fatal("Failed to setup chrome: ", err)
			}
			defer closeBrowser(cleanupCtx)

			defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			// Open the test page.
			conn, err := br.NewConn(ctx, server.URL+"/notifications_for_urls_test_page.html")
			if err != nil {
				s.Fatal("Failed to connect to the policy page: ", err)
			}
			defer conn.Close()

			var notificationPermission string
			if err := conn.Eval(ctx, `Notification.permission`, &notificationPermission); err != nil {
				s.Fatal("Could not read notification permission: ", err)
			}

			if notificationPermission != "granted" && notificationPermission != "denied" && notificationPermission != "default" {
				s.Fatal("Unable to capture Notification Setting")
			}
			notificationBlocked := notificationPermission == "denied"
			// Check if the notification permission is inline with the expected permission.
			if notificationBlocked != param.notificationBlocked {
				s.Errorf("Unexpected permission for notifications: got %v; want %v", notificationBlocked, param.notificationBlocked)
			}
		})
	}
}
