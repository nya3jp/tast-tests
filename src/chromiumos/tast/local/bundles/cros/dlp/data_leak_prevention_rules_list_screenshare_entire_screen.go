// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/local/bundles/cros/dlp/restrictionlevel"
	"chromiumos/tast/local/bundles/cros/dlp/screenshare"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/webutil"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListScreenshareEntireScreen,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with screen sharing restrictions while sharing an entire screen",
		Contacts: []string{
			"ayaelattar@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Data:         []string{"text_1.html", "text_2.html"},
		Params: []testing.Param{{
			Name:    "ash_blocked",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshare.TestParams{
				Name:        "blocked",
				Restriction: restrictionlevel.Blocked,
				Path:        screenshare.RestrictedPath,
				BrowserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_allowed",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshare.TestParams{
				Name:        "allowed",
				Restriction: restrictionlevel.Allowed,
				Path:        screenshare.UnrestrictedPath,
				BrowserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_proceeded",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshare.TestParams{
				Name:        "warn_proceeded",
				Restriction: restrictionlevel.WarnProceeded,
				Path:        screenshare.RestrictedPath,
				BrowserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_cancelled",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshare.TestParams{
				Name:        "warn_cancelled",
				Restriction: restrictionlevel.WarnCancelled,
				Path:        screenshare.RestrictedPath,
				BrowserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_blocked",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshare.TestParams{
				Name:        "blocked",
				Restriction: restrictionlevel.Blocked,
				Path:        screenshare.RestrictedPath,
				BrowserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_allowed",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshare.TestParams{
				Name:        "allowed",
				Restriction: restrictionlevel.Allowed,
				Path:        screenshare.UnrestrictedPath,
				BrowserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_proceeded",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshare.TestParams{
				Name:        "warn_proceeded",
				Restriction: restrictionlevel.WarnProceeded,
				Path:        screenshare.RestrictedPath,
				BrowserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_cancelled",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshare.TestParams{
				Name:        "warn_cancelled",
				Restriction: restrictionlevel.WarnCancelled,
				Path:        screenshare.RestrictedPath,
				BrowserType: browser.TypeLacros,
			},
		},
		}})
}

func DataLeakPreventionRulesListScreenshareEntireScreen(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	params := s.Param().(screenshare.TestParams)

	server := httptest.NewServer(http.FileServer(s.DataFileSystem()))
	defer server.Close()

	url := server.URL + params.Path
	unrestrictedURL := server.URL + screenshare.UnrestrictedPath

	// Update the policy blob.
	pb := policy.NewBlob()
	if params.Restriction == restrictionlevel.Allowed || params.Restriction == restrictionlevel.Blocked {
		pb.AddPolicies(screenshare.GetScreenshareBlockPolicy(server.URL))
	} else {
		pb.AddPolicies(screenshare.GetScreenshareWarnPolicy(server.URL))
	}

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

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

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, params.BrowserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	conn, err := br.NewConn(ctx, unrestrictedURL)
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatalf("Failed to wait for %q to achieve quiescence: %v", unrestrictedURL, err)
	}

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+params.Name)

	var screenRecorder *uiauto.ScreenRecorder
	screenRecorder, err = uiauto.NewScreenRecorder(ctx, tconn)

	if err != nil {
		s.Fatal("Failed to create ScreenRecorder: ", err)
	}

	if screenRecorder == nil {
		s.Fatal("Screen recorder was not found")
	}

	if err := screenRecorder.Start(ctx, tconn); err != nil {
		s.Fatal("Failed to start screen recorder: ", err)
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "dlpScreenShare.mp4"))

	wantAllowed := params.Restriction == restrictionlevel.Allowed || params.Restriction == restrictionlevel.WarnProceeded

	// Screenshare should be allowed.
	if err := screenshare.CheckFrameStatus(ctx, screenRecorder, true); err != nil {
		s.Fatal("Failed to check frame status: ", err)
	}

	conn, err = br.NewConn(ctx, url)
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatalf("Failed to wait for %q to achieve quiescence: %v", url, err)
	}

	// The "Screen share paused" notification should appear if and only if the site is blocked.
	if _, err := ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(screenshare.ScreensharePausedIDContains), ash.WaitTitle(screenshare.ScreensharePausedTitle)); (err != nil) == (params.Restriction == restrictionlevel.Blocked) {
		if err != nil {
			s.Errorf("Failed to wait for notification with title %q: %v", screenshare.ScreensharePausedTitle, err)
		} else {
			s.Errorf("Notification with title %q appeared when it should not have", screenshare.ScreensharePausedTitle)
		}
	}

	if params.Restriction == restrictionlevel.WarnProceeded {
		// Hit Enter, which is equivalent to clicking on the "Share anyway" button.
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			s.Fatal("Failed to hit Enter: ", err)
		}

		if _, err := ash.WaitForNotification(ctx, tconn, 10*time.Second, ash.WaitIDContains(screenshare.ScreenshareResumedIDContains), ash.WaitTitle(screenshare.ScreenshareResumedTitle)); err != nil {
			s.Errorf("Failed to wait for notification with title %q: %v", screenshare.ScreenshareResumedTitle, err)
		}

	} else if params.Restriction == restrictionlevel.WarnCancelled {
		// Hit Esc, which is equivalent to clicking on the "Cancel" button.
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			s.Fatal("Failed to hit Esc: ", err)
		}
	}

	// Close notifications.
	if err := ash.CloseNotifications(ctx, tconn); err != nil {
		s.Fatal("Failed to close notifications: ", err)
	}

	// Frame status value should be as per wantAllowed.
	if err := screenshare.CheckFrameStatus(ctx, screenRecorder, wantAllowed); err != nil {
		s.Fatal("Polling the frame status timed out: ", err)
	}

	conn, err = br.NewConn(ctx, unrestrictedURL)
	if err != nil {
		s.Fatal("Failed to open page: ", err)
	}
	defer conn.Close()

	if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
		s.Fatalf("Failed to wait for %q to achieve quiescence: %v", unrestrictedURL, err)
	}

	if _, err := ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(screenshare.ScreenshareResumedIDContains), ash.WaitTitle(screenshare.ScreenshareResumedTitle)); (err != nil) == (params.Restriction == restrictionlevel.Blocked) {
		if err != nil {
			s.Errorf("Failed to wait for notification with title %q: %v", screenshare.ScreenshareResumedTitle, err)
		} else {
			s.Errorf("Notification with title %q appeared when it should not have", screenshare.ScreenshareResumedTitle)
		}
	}

	// Screenshare should be allowed unless user cancelled sharing after a warning.
	if err := screenshare.CheckFrameStatus(ctx, screenRecorder, params.Restriction != restrictionlevel.WarnCancelled); err != nil {
		s.Fatal("Failed to check frame status: ", err)
	}

	// Once the user clicks "Share anyway", returning to the site later should allow for sharing without another prompt.
	if params.Restriction == restrictionlevel.WarnProceeded {
		if conn, err = br.NewConn(ctx, url); err != nil {
			s.Fatal("Failed to open page: ", err)
		}

		if err := webutil.WaitForQuiescence(ctx, conn, 10*time.Second); err != nil {
			s.Fatalf("Failed to wait for %q to achieve quiescence: %v", unrestrictedURL, err)
		}

		if err := screenshare.CheckFrameStatus(ctx, screenRecorder /*wantAllowed=*/, true); err != nil {
			s.Fatal("Failed to check frame status: ", err)
		}
	}
}
