// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"path/filepath"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListScreenshare,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with screenshare blocked restriction",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListScreenshare(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// DLP policy with screenshare blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable screenshare of confidential content",
				Description: "User should not be able to screen share confidential content",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "SCREEN_SHARE",
						Level: "BLOCK",
					},
				},
			},
		},
	},
	}

	// Update the policy blob.
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(policyDLP)

	// Update policy.
	if err := policyutil.ServeBlobAndRefresh(ctx, fakeDMS, cr, pb); err != nil {
		s.Fatal("Failed to serve and refresh: ", err)
	}

	// Connect to Test API.
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to connect to test API: ", err)
	}

	screenRecorder, err := uiauto.NewScreenRecorder(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to create ScreenRecorder: ", err)
	}

	if screenRecorder == nil {
		s.Fatal("Screen recorder was not found")
	}

	defer uiauto.ScreenRecorderStopSaveRelease(ctx, screenRecorder, filepath.Join(s.OutDir(), "dlpScreenShare.mp4"))

	screenRecorder.Start(ctx, tconn)

	const paused = "Screen capture paused"
	const resumed = "Screen capture resumed"
	const nonRestrictedSite = "https://www.chromium.org/"

	for _, param := range []struct {
		name        string
		wantAllowed bool
		url         string
	}{
		{
			name:        "example",
			wantAllowed: false,
			url:         "https://www.example.com/",
		},
		{
			name:        "company",
			wantAllowed: true,
			url:         "https://www.company.com/",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if _, err = cr.NewConn(ctx, nonRestrictedSite); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := checkFrameStatus(ctx, screenRecorder, true); err != nil {
				s.Fatal("Failed to check frame status: ", err)
			}

			if _, err = cr.NewConn(ctx, param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := checkFrameStatus(ctx, screenRecorder, param.wantAllowed); err != nil {
				s.Fatal("Failed to check frame status: ", err)
			}

			if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("screen_capture_dlp_paused-"), ash.WaitTitle(paused)); err != nil && !param.wantAllowed {
				s.Fatalf("Failed to wait for notification with title %q: %v", paused, err)
			}

			if _, err = cr.NewConn(ctx, nonRestrictedSite); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := checkFrameStatus(ctx, screenRecorder, true); err != nil {
				s.Fatal("Failed to check frame status: ", err)
			}

			if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("screen_capture_dlp_resumed-"), ash.WaitTitle(resumed)); err != nil && !param.wantAllowed {
				s.Fatalf("Failed to wait for notification with title %q: %v", resumed, err)
			}

			if _, err := ash.WaitForNotification(ctx, tconn, 15*time.Second, ash.WaitIDContains("screen_capture_dlp_paused-"), ash.WaitTitle(paused)); err == nil {
				s.Fatalf("Notification with title %q found, expected none", paused)
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
		})
	}

}

func checkFrameStatus(ctx context.Context, screenRecorder *uiauto.ScreenRecorder, wantAllowed bool) error {
	if screenRecorder == nil {
		return errors.New("couldn't check frame status. Screen recorder was not found")
	}

	status, err := screenRecorder.FrameStatus(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get status of frame: ")
	}

	if status != "Success" && wantAllowed {
		return errors.Errorf("Frame not recording. got: %v, want Success", status)
	}

	if status != "Fail" && !wantAllowed {
		return errors.Errorf("Frame recording. got: %v, want Fail", status)
	}

	return nil
}
