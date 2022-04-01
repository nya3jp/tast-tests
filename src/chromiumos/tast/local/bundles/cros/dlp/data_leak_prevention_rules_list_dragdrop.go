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
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/chrome/webutil"
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

	defer ash.SetOverviewModeAndWait(ctx, tconn, false)
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

			conn1, err := cr.NewConn(ctx, "https://www.google.com/")
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}

			if err := webutil.WaitForQuiescence(ctx, conn1, 10*time.Second); err != nil {
				s.Fatal("Failed to wait for google.com to achieve quiescence: ", err)
			}

			conn2, err := cr.NewConn(ctx, "https://"+param.url, browser.WithNewWindow())
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}

			if err := webutil.WaitForQuiescence(ctx, conn2, 10*time.Second); err != nil {
				s.Fatalf("Failed to wait for %q to achieve quiescence: %v", param.url, err)
			}

			if err := ash.SetOverviewModeAndWait(ctx, tconn, true); err != nil {
				s.Fatal("Failed to enter into the overview mode: ", err)
			}

			// Snap the param.url window to the right
			w1, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatalf("Failed to find the %s window in the overview mode: %s", param.url, err)
			}

			if err := ash.SetWindowStateAndWait(ctx, tconn, w1.ID, ash.WindowStateRightSnapped); err != nil {
				s.Fatalf("Failed to snap the %s window to the right: %s", param.url, err)
			}

			// Snap the google.com window to the left.
			w2, err := ash.FindFirstWindowInOverview(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to find the google.com window in the overview mode: ", err)
			}

			if err := ash.SetWindowStateAndWait(ctx, tconn, w2.ID, ash.WindowStateLeftSnapped); err != nil {
				s.Fatal("Failed to snap the google.com window to the left: ", err)
			}

			// Activate the clipboard source (param.url) window.
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

			// Verify notification bubble.
			notifError := clipboard.CheckClipboardBubble(ctx, ui, param.url)

			if !param.wantAllowed && notifError != nil {
				s.Error("Expected notification but found an error: ", notifError)
			}

			if param.wantAllowed && notifError == nil {
				s.Error("Didn't expect notification but one was found: ")
			}

			// Check dropped content.
			dropError := checkDraggedContent(ctx, ui, param.content)

			if param.wantAllowed && dropError != nil {
				s.Error("Checked pasted content but found an error: ", dropError)
			}

			if !param.wantAllowed && dropError == nil {
				s.Error("Content was pasted but should have been blocked")
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

	searchTab := nodewith.Name("Search").Role(role.TextFieldWithComboBox).State(state.Editable, true).First()
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

// checkDraggedContent checks if a certain |content| appears in the search box.
func checkDraggedContent(ctx context.Context, ui *uiauto.Context, content string) error {
	contentNode := nodewith.NameContaining(content).Role(role.InlineTextBox).State(state.Editable, true).First()

	if err := ui.WaitUntilExists(contentNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to check for dragged content")
	}

	return nil
}
