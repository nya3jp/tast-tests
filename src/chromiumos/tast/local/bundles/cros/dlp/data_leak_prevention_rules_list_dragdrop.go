// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"
	"time"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         DataLeakPreventionRulesListDragdrop,
		LacrosStatus: testing.LacrosVariantUnknown,
		Desc:         "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction by drag and drop",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListDragdrop(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(chrome.HasChrome).Chrome()
	fakeDMS := s.FixtValue().(fakedms.HasFakeDMS).FakeDMS()

	// Set DLP policy with clipboard blocked restriction.
	if err := policyutil.ServeAndVerify(ctx, fakeDMS, cr, policy.StandardDLPPolicyForClipboard()); err != nil {
		s.Fatal("Failed to serve and verify: ", err)
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

	for _, param := range []struct {
		name        string
		wantAllowed bool
		url         string
		content     string
	}{
		{
			name:        "example",
			wantAllowed: false,
			url:         "www.example.com",
			content:     "Example Domain",
		},
		{
			name:        "chromium",
			wantAllowed: true,
			url:         "www.chromium.org",
			content:     "The Chromium Projects",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := cr.ResetState(ctx); err != nil {
				s.Fatal("Failed to reset the Chrome: ", err)
			}

			if _, err = cr.NewConn(ctx, "https://www.google.com/"); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if _, err = cr.NewConn(ctx, "https://"+param.url, browser.WithNewWindow()); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}

			// Snap param.url window to the right
			w1, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatalf("Failed to find the %s window in the overview mode: %s", param.url, err)
			}

			// w1.ID supposed to have the window id which is right-snapped.
			if err := ash.SetWindowStateAndWait(ctx, tconn, w1.ID, ash.WindowStateRightSnapped); err != nil {
				s.Fatalf("Failed to snap the %s window: %s", param.url, err)
			}

			// Snap the google.com window to the left.
			w2, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to find the google.com window in the overview mode: ", err)
			}

			// w2.ID supposed to have the window id of google.com which is left-snapped.
			if err := ash.SetWindowStateAndWait(ctx, tconn, w2.ID, ash.WindowStateLeftSnapped); err != nil {
				s.Fatal("Failed to snap the google.com window: ", err)
			}

			if err := w1.ActivateWindow(ctx, tconn); err != nil {
				s.Fatalf("Failed to activate the %s window: %s", param.url, err)
			}

			if err = keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			s.Log("Draging and dropping content")
			if err := dragDrop(ctx, tconn, param.content); err != nil {
				s.Error("Failed to drag drop content: ", err)
			}

			s.Log("Checking notification")
			ui := uiauto.New(tconn)
			err = clipboard.CheckClipboardBubble(ctx, ui, param.url)

			if !param.wantAllowed && err != nil {
				s.Error("Couldn't check for notification: ", err)
			}

			if param.wantAllowed && err == nil {
				s.Error("Content pasted, expected restriction")
			}
		})
	}
}

func dragDrop(ctx context.Context, tconn *chrome.TestConn, content string) error {
	ui := uiauto.New(tconn)

	contentNode := nodewith.Name(content).First()
	start, err := ui.Location(ctx, contentNode)
	if err != nil {
		return errors.Wrap(err, "failed to get locaton for content")
	}

	search := "Google Search"
	searchTab := nodewith.Name(search).Role(role.InlineTextBox).First()
	endLocation, err := ui.Location(ctx, searchTab)
	if err != nil {
		return errors.Wrap(err, "failed to get locaton for google search")
	}

	if err := uiauto.Combine("Drag and Drop",
		mouse.Drag(tconn, start.CenterPoint(), endLocation.CenterPoint(), time.Second*2))(ctx); err != nil {
		return errors.Wrap(err, "failed to verify content preview for")
	}
	return nil
}
