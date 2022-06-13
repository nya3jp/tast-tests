// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/bundles/cros/dlp/restrictionlevel"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/screenshot"
	"chromiumos/tast/testing"
)

// Notification strings.
const (
	captureNotAllowedTitle = "Can't capture confidential content"
	captureNotAllowedID    = "screen_capture_dlp_blocked"
	captureTakenTitle      = "Screenshot taken"
	captureTakenID         = "capture_mode_notification"
)

// A struct containing parameters for different screenshot tests.
type screenshotTestParams struct {
	name                  string
	path                  string
	restriction           restrictionlevel.RestrictionLevel
	wantNotificationTitle string
	wantNotificationID    string
	browserType           browser.Type
}

// Different paths used for testing.
const (
	unrestrictedPath = "/text_1.html"
	restrictedPath   = "/text_2.html"
)

// getScreenshotBlockPolicy returns a DLP policy that blocks screen sharing.
func getScreenshotBlockPolicy(serverURL string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable taking screenshots of confidential content",
				Description: "User should not be able to take screenshots of confidential content",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						serverURL + restrictedPath,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "SCREENSHOT",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}
}

// getScreenshotWarnPolicy returns a DLP policy that warns before taking a screenshot.
func getScreenshotWarnPolicy(serverURL string) []policy.Policy {
	return []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Warn before taking a screenshot confidential content",
				Description: "User should be warned before taking a screenshot of confidential content",
				Sources: &policy.DataLeakPreventionRulesListValueSources{
					Urls: []string{
						serverURL + restrictedPath,
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
					{
						Class: "SCREENSHOT",
						Level: "WARN",
					},
				},
			},
		},
	},
	}
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListScreenshot,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with screenshot restrictions",
		Contacts: []string{
			"ayaelattar@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline"},
		Data:         []string{"text_1.html", "text_2.html"},
		Params: []testing.Param{{
			Name:    "ash_blocked",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshotTestParams{
				name:                  "blocked",
				path:                  restrictedPath,
				restriction:           restrictionlevel.Blocked,
				wantNotificationTitle: captureNotAllowedTitle,
				wantNotificationID:    captureNotAllowedID,
				browserType:           browser.TypeAsh,
			},
		}, {
			Name:    "ash_allowed",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshotTestParams{
				name:                  "allowed",
				path:                  unrestrictedPath,
				restriction:           restrictionlevel.Allowed,
				wantNotificationTitle: captureTakenTitle,
				wantNotificationID:    captureTakenID,
				browserType:           browser.TypeAsh,
			},
		}, {
			Name:      "ash_warn_proceeded",
			ExtraAttr: []string{"informational"},
			Fixture:   fixture.ChromePolicyLoggedIn,
			Val: screenshotTestParams{
				name:                  "warn_proceded",
				path:                  restrictedPath,
				restriction:           restrictionlevel.WarnProceeded,
				wantNotificationTitle: captureTakenTitle,
				wantNotificationID:    captureTakenID,
				browserType:           browser.TypeAsh,
			},
		}, {
			Name:      "ash_warn_cancelled",
			ExtraAttr: []string{"informational"},
			Fixture:   fixture.ChromePolicyLoggedIn,
			Val: screenshotTestParams{
				name:        "warn_cancelled",
				path:        restrictedPath,
				restriction: restrictionlevel.WarnCancelled,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_blocked",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshotTestParams{
				name:                  "blocked",
				path:                  restrictedPath,
				restriction:           restrictionlevel.Blocked,
				wantNotificationTitle: captureNotAllowedTitle,
				wantNotificationID:    captureNotAllowedID,
				browserType:           browser.TypeLacros,
			},
		}, {
			Name:              "lacros_allowed",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshotTestParams{
				name:                  "allowed",
				path:                  UnrestrictedPath,
				restriction:           restrictionlevel.Allowed,
				wantNotificationTitle: captureTakenTitle,
				wantNotificationID:    captureTakenID,
				browserType:           browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_proceeded",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshotTestParams{
				name:                  "warn_proceeded",
				path:                  restrictedPath,
				restriction:           restrictionlevel.WarnProceeded,
				wantNotificationTitle: captureTakenTitle,
				wantNotificationID:    captureTakenID,
				browserType:           browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_cancelled",
			ExtraAttr:         []string{"informational"},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshotTestParams{
				name:        "warn_cancelled",
				path:        restrictedPath,
				restriction: restrictionlevel.WarnCancelled,
				browserType: browser.TypeLacros,
			},
		}},
	})
}

func DataLeakPreventionRulesListScreenshot(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 10*time.Second)
	defer cancel()

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	defer faillog.DumpUITreeWithScreenshotOnError(cleanupCtx, s.OutDir(), s.HasError, cr, "ui_tree_"+s.Param().(screenshotTestParams).name)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	// Update the policy blob.
	pb := policy.NewBlob()
	if s.Param().(screenshotTestParams).restriction == restrictionlevel.Allowed || s.Param().(screenshotTestParams).restriction == restrictionlevel.Blocked {
		pb.AddPolicies(getScreenshotBlockPolicy(server.URL))
	} else {
		pb.AddPolicies(getScreenshotWarnPolicy(server.URL))
	}

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	// Close previous notifications.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	// Clean up previous screenshots.
	if err := screenshot.RemoveScreenshots(); err != nil {
		s.Fatal("Failed to remove screenshots: ", err)
	}

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(screenshotTestParams).browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	url := server.URL + s.Param().(screenshotTestParams).path
	conn, err := br.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer conn.Close()

	testScreenshot(ctx, s, tconn, keyboard, s.Param().(screenshotTestParams))
}

// testScreenshot attempts to take a screenshot, and reports errors if the behavior is different than expected.
func testScreenshot(ctx context.Context, s *testing.State, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, params screenshotTestParams) {
	// Press Ctrl+F5 to take the screenshot.
	if err := keyboard.Accel(ctx, "Ctrl+F5"); err != nil {
		s.Fatal("Failed to type screenshot hotkey: ", err)
	}

	if params.restriction == restrictionlevel.WarnProceeded {
		// Hit Enter, which is equivalent to clicking on the "Capture anyway" button.
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			s.Fatal("Failed to hit Enter: ", err)
		}
	}

	if params.restriction == restrictionlevel.WarnCancelled {
		// Hit Esc, which is equivalent to clicking on the "Cancel" button.
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			s.Fatal("Failed to hit Esc: ", err)
		}

		// Ensure that a screenshot was not taken.
		if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains(captureTakenID), ash.WaitTitle(captureTakenTitle)); err == nil {
			s.Error("\"" + captureTakenTitle + "\" notification appeared after user cancelled action")
		}
	} else {
		// Restriction levels other than WarnCancelled should all result in a notification appearing, either for the screenshot being taken or blocked.
		if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains(params.wantNotificationID), ash.WaitTitle(params.wantNotificationTitle)); err != nil {
			s.Errorf("Failed to wait for notification with title \"%q\": %v", params.wantNotificationTitle, err)
		}
	}

	// Check if a screenshot was taken.
	takenScreenshot, err := screenshot.HasScreenshots()
	if err != nil {
		s.Fatal("Failed to check if a screenshot was taken: ", err)
	}

	wantScreenshotTaken := (params.restriction == restrictionlevel.Allowed) || (params.restriction == restrictionlevel.WarnProceeded)

	if takenScreenshot != wantScreenshotTaken {
		s.Errorf("Unexpected screenshot allowed: got %v; want %v", takenScreenshot, wantScreenshotTaken)
	}
}
