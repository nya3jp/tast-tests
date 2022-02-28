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

// A struct containing parameters for different screenshare tests.
type screenshareTestParams struct {
	name        string
	url         string
	restriction restrictionlevel.RestrictionLevel
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
					"example.com",
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
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Params: []testing.Param{{
			Name:    "ash_blocked",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "blocked",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.Blocked,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_allowed",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "allowed",
				url:         "https://www.chromium.org/",
				restriction: restrictionlevel.Allowed,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_proceeded",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_proceded",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnProceeded,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:    "ash_warn_cancelled",
			Fixture: fixture.ChromePolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_cancelled",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnCancelled,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeAsh,
			},
		}, {
			Name:              "lacros_blocked",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "blocked",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.Blocked,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_allowed",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "allowed",
				url:         "https://www.chromium.org/",
				restriction: restrictionlevel.Allowed,
				policyDLP:   screenshareBlockPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_proceeded",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_proceded",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnProceeded,
				policyDLP:   screenshareWarnPolicy,
				browserType: browser.TypeLacros,
			},
		}, {
			Name:              "lacros_warn_cancelled",
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           fixture.LacrosPolicyLoggedIn,
			Val: screenshareTestParams{
				name:        "warn_cancelled",
				url:         "https://www.example.com/",
				restriction: restrictionlevel.WarnCancelled,
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
	pb := fakedms.NewPolicyBlob()
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

	// Start screen recorder.
	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create ScreenRecorder: ", err)
	}

	if screenRecorder == nil {
		s.Fatal("Screen recorder was not found")
	}
	screenRecorder.Start(ctx, tconn)
	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "dlpScreenShare.mp4"))

	const nonRestrictedSite = "https://www.chromium.org/"
	wantAllowed := params.restriction == restrictionlevel.Allowed || params.restriction == restrictionlevel.WarnProceeded

	defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+params.name)

	if _, err = cr.NewConn(ctx, nonRestrictedSite); err != nil {
		s.Fatal("Failed to open page: ", err)
	}

	// Screenshare should be allowed.
	if err := checkFrameStatus(ctx, screenRecorder, true); err != nil {
		s.Fatal("Failed to check frame status: ", err)
	}

	if _, err = cr.NewConn(ctx, params.url); err != nil {
		s.Fatal("Failed to open page: ", err)
	}

	if params.restriction == restrictionlevel.WarnProceeded {
		// Hit Enter, which is equivalent to clicking on the "Share anyway" button.
		if err := keyboard.Accel(ctx, "Enter"); err != nil {
			s.Fatal("Failed to hit Enter: ", err)
		}

		// Continuing screen sharing should result in a "Screen share resumed" notification.
		if _, err := ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(screenshareResumedIDContains), ash.WaitTitle(screenshareResumedTitle)); err != nil {
			s.Errorf("Failed to wait for notification with title %q: %v", screenshareResumedTitle, err)
		}
	} else if params.restriction == restrictionlevel.WarnCancelled {
		// Hit Esc, which is equivalent to clicking on the "Cancel" button.
		if err := keyboard.Accel(ctx, "Esc"); err != nil {
			s.Fatal("Failed to hit Esc: ", err)
		}
	}

	// Frame status value should be as per wantAllowed.
	if err := checkFrameStatus(ctx, screenRecorder, wantAllowed); err != nil {
		s.Fatal("Failed to check frame status: ", err)
	}

	// The "Screen share paused" notification should only appear if the site is blocked.
	if _, err := ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(screensharePausedIDContains), ash.WaitTitle(screensharePausedTitle)); err != nil && params.restriction == restrictionlevel.Blocked {
		s.Errorf("Failed to wait for notification with title %q: %v", screensharePausedTitle, err)
	}

	if _, err = cr.NewConn(ctx, nonRestrictedSite); err != nil {
		s.Fatal("Failed to open page: ", err)
	}

	// Screen sharing should still be active unless user cancelled sharing.
	if err := checkFrameStatus(ctx, screenRecorder, params.restriction != restrictionlevel.WarnCancelled); err != nil {
		s.Fatal("Failed to check frame status: ", err)
	}

	if _, err := ash.WaitForNotification(ctx, tconn, 5*time.Second, ash.WaitIDContains(screenshareResumedIDContains), ash.WaitTitle(screenshareResumedTitle)); err != nil && params.restriction == restrictionlevel.Blocked {
		s.Errorf("Failed to wait for notification with title %q: %v", screenshareResumedTitle, err)
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
