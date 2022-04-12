// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/fixture"
	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/restrictionlevel"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

// Screen share notification strings.
const (
	screensharePausedTitle       = "Screen share paused"
	screensharePausedIDContains  = "screen_share_dlp_paused-"
	screenshareResumedTitle      = "Screen share resumed"
	screenshareResumedIDContains = "screen_share_dlp_resumed-"
)

// screenshareTarget is an enum containing different possible ways a user can share their screen.
type screenshareMediaStreamType int

// See comment on the type above.
const (
	screen screenshareMediaStreamType = iota
	window
)

// A struct containing parameters for different screenshare tests.
type screenshareTestParams struct {
	name        string
	url         string
	restriction restrictionlevel.RestrictionLevel
	target      screenshareMediaStreamType
	policyDLP   []policy.Policy
	browserType browser.Type
}

// DLP policy that blocks screen sharing.
var screenshareBlockPolicy = []policy.Policy{&policy.DataLeakPreventionRulesList{
	Val: []*policy.DataLeakPreventionRulesListValue{
		{
			Name:        "Disable sharing the screen with confidential content visible",
			Description: "User should not be able to share the screen with confidential content visible",
			Sources: &policy.DataLeakPreventionRulesListValueSources{
				Urls: []string{
					"example.com",
				},
			},
			Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
				{
					Class: "SCREEN_SHARE",
					Level: "BLOCK",
				},
			},
		},
	},
},
}

// DLP policy that warns when trying to share the screen.
var screenshareWarnPolicy = []policy.Policy{&policy.DataLeakPreventionRulesList{
	Val: []*policy.DataLeakPreventionRulesListValue{
		{
			Name:        "Warn before sharing the screen with confidential content visible",
			Description: "User should be warned before sharing the screen with confidential content visible",
			Sources: &policy.DataLeakPreventionRulesListValueSources{
				Urls: []string{
					// We specify 2 URLs because different *warn_proceeded* tests need to
					// use different URLs, as clicking on "Share anyway" in one test means
					// the user would not be prompted in subsequent ones, and sharing would
					// automatically be allowed for that URL.
					"example.com",
					"google.com",
				},
			},
			Restrictions: []*policy.DataLeakPreventionRulesListValueRestrictions{
				{
					Class: "SCREEN_SHARE",
					Level: "WARN",
				},
			},
		},
	},
},
}

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListScreenshare,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with screen sharing restrictions",
		Contacts: []string{
			"ayaelattar@google.com",
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "ash_blocked_screen_share",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "blocked_screen_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.Blocked,
				target:      screen,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_allowed_screen_share",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "allowed_screen_share",
				url:         "https://www.chromium.org/",
				restriction: restrictionlevel.Allowed,
				target:      screen,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_proceeded_screen_share",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_proceded_screen_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnProceeded,
				target:      screen,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_cancelled_screen_share",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_cancelled_screen_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnCancelled,
				target:      screen,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_blocked_screen_share",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "blocked_screen_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.Blocked,
				target:      screen,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_allowed_screen_share",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "allowed_screen_share",
				url:         "https://www.chromium.org/",
				restriction: restrictionlevel.Allowed,
				target:      screen,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_proceeded_screen_share",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_proceded_screen_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnProceeded,
				target:      screen,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_cancelled_screen_share",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_cancelled_screen_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnCancelled,
				target:      screen,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:    "ash_blocked_window_share",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "blocked_window_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.Blocked,
				target:      window,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_allowed_window_share",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "allowed_window_share",
				url:         "https://www.chromium.org/",
				restriction: restrictionlevel.Allowed,
				target:      window,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_proceeded_window_share",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_proceded_window_share",
				url:         "https://www.google.com/",
				restriction: restrictionlevel.WarnProceeded,
				target:      window,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_cancelled_window_share",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_cancelled_window_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnCancelled,
				target:      window,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_blocked_window_share",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "blocked_window_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.Blocked,
				target:      window,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_allowed_window_share",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "allowed_window_share",
				url:         "https://www.chromium.org/",
				restriction: restrictionlevel.Allowed,
				target:      window,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_proceeded_window_share",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_proceded_window_share",
				url:         "https://www.google.com/",
				restriction: restrictionlevel.WarnProceeded,
				target:      window,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_cancelled_window_share",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_cancelled_window_share",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnCancelled,
				target:      window,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeLacros,
			},
		},
		}})
}

func DataLeakPreventionRulesListScreenshare(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()
	params := s.Param().(screenshareTestParams)

	// Update the policy blob.
	pb := policy.NewBlob()
	pb.AddPolicies(params.policyDLP)

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

	br, closeBrowser, err := browserfixt.SetUp(ctx, s.FixtValue(), params.browserType)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	const nonRestrictedSite = "https://www.chromium.org/"
	if _, err := br.NewConn(ctx, nonRestrictedSite); err != nil {
		s.Fatal("Failed to open page: ", err)
	}

	var screenRecorder *uiauto.ScreenRecorder

	if params.target == screen {
		screenRecorder, err = uiauto.NewScreenRecorder(ctx, tconn)
	} else if params.target == window {
		screenRecorder, err = uiauto.NewWindowRecorder(ctx, tconn /*windowIndex=*/, 0)
	}

	if err != nil {
		s.Fatal("Failed to create ScreenRecorder: ", err)
	}

	if screenRecorder == nil {
		s.Fatal("Screen recorder was not found")
	}
	screenRecorder.Start(ctx, tconn)
	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "dlpScreenShare.mp4"))

	wantAllowed := params.restriction == restrictionlevel.Allowed || params.restriction == restrictionlevel.WarnProceeded

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+params.name)

	// Screenshare should be allowed.
	if err := checkFrameStatus(ctx, screenRecorder, true); err != nil {
		s.Fatal("Failed to check frame status: ", err)
	}

	if _, err = br.NewConn(ctx, params.url); err != nil {
		s.Fatal("Failed to open page: ", err)
	}

	// The "Screen share paused" notification should appear iff the site is blocked.
	if _, err := ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(screensharePausedIDContains), ash.WaitTitle(screensharePausedTitle)); (err != nil) == (params.restriction == restrictionlevel.Blocked) {
		if err != nil {
			s.Errorf("Failed to wait for notification with title %q: %v", screensharePausedTitle, err)
		} else {
			s.Errorf("Notification with title %q appeared when it should not have", screensharePausedTitle)
		}
	}

	if params.restriction == restrictionlevel.WarnProceeded {
		// Hit Enter, which is equivalent to clicking on the "Share anyway" button.
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			s.Fatal("Failed to hit Enter: ", err)
		}

		// Continuing screen sharing should result in a "Screen share resumed" notification.
		if _, err := ash.WaitForNotification(ctx, tconn, 10*time.Second, ash.WaitIDContains(screenshareResumedIDContains), ash.WaitTitle(screenshareResumedTitle)); err != nil {
			s.Errorf("Failed to wait for notification with title %q: %v", screenshareResumedTitle, err)
		}
	} else if params.restriction == restrictionlevel.WarnCancelled {
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
	// This check can sometimes randomly fail even if the screen is being shared, so we poll.
	if err := testing.Poll(ctx, func(ctx context.Context) error {
		if err := checkFrameStatus(ctx, screenRecorder, wantAllowed); err != nil {
			return errors.Wrap(err, "failed to check frame status")
		}

		return nil
	}, &testing.PollOptions{Timeout: 10 * time.Second, Interval: 500 * time.Millisecond}); err != nil {
		s.Fatal("Polling the frame status timed out: ", err)
	}

	if _, err = br.NewConn(ctx, nonRestrictedSite); err != nil {
		s.Fatal("Failed to open page: ", err)
	}

	// If screen sharing was blocked, the "screenshare resumed" notification should appear now, having navigated to a non-restricted site. Otherwise, the notification should not appear.
	if _, err := ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(screenshareResumedIDContains), ash.WaitTitle(screenshareResumedTitle)); (err != nil) == (params.restriction == restrictionlevel.Blocked) {
		if err != nil {
			s.Errorf("Failed to wait for notification with title %q: %v", screenshareResumedTitle, err)
		} else {
			s.Errorf("Notification with title %q appeared when it should not have", screenshareResumedTitle)
		}
	}

	// Closing all windows.
	ws, err := ash.GetAllWindows(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to get all open windows: ", err)
	}
	for _, w := range ws {
		if err := w.CloseWindow(ctx, tconn); err != nil {
			s.Logf("Warning: Failed to close window (%+v): %v", w, err)
		}
	}

}

func checkFrameStatus(ctx context.Context, screenRecorder *uiauto.ScreenRecorder, wantAllowed bool) error {
	if screenRecorder == nil {
		return errors.New("couldn't check frame status. Screen recorder was not found")
	}

	status, err := screenRecorder.FrameStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get status of frame")
	}

	if status != "Success" && wantAllowed {
		return errors.Errorf("Frame not recording. got: %v, want Success", status)
	}

	if status != "Fail" && !wantAllowed {
		return errors.Errorf("Frame recording. got: %v, want Fail", status)
	}

	return nil
}
