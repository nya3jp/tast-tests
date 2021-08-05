// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package dlp

import (
	"context"

	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/bundles/cros/dlp/clipboard"
	"chromiumos/tast/local/bundles/cros/dlp/policy"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListClipboardShelf,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction in the shelf textfield",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListClipboardShelf(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// DLP policy with all clipboard blocked restriction.
	policyDLP := policy.RestrictiveDLPPolicyForClipboard()

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

	keyboard, err := input.VirtualKeyboard(ctx)
	if err != nil {
		s.Fatal("Failed to get keyboard: ", err)
	}
	defer keyboard.Close()

	s.Log("Waiting for chrome.clipboard API to become available")
	if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		s.Fatal("chrome.clipboard API unavailable: ", err)
	}

	for _, param := range []struct {
		name        string
		url         string
		wantAllowed bool
	}{
		{
			name:        "example",
			url:         "www.example.com",
			wantAllowed: false,
		},
		{
			name:        "chromium",
			url:         "www.chromium.org",
			wantAllowed: true,
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if err := cr.ResetState(ctx); err != nil {
				s.Fatal("Failed to reset the Chrome: ", err)
			}

			if _, err = cr.NewConn(ctx, "https://"+param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			// Press the search key to bring the launcher into focus.
			if err := keyboard.Accel(ctx, "Search"); err != nil {
				s.Fatal("Failed to press Search to open shelf box: ", err)
			}

			s.Log("Right clicking shelf box")
			if err := rightClickShelfbox(ctx, tconn, param.url, param.wantAllowed); err != nil {
				s.Error("Failed to right click shelf box: ", err)
			}

			s.Log("Pasting content in shelf box")
			if err := pasteShelfbox(ctx, tconn, keyboard, param.url, param.wantAllowed); err != nil {
				s.Error("Failed to paste content in shelf box: ", err)
			}
		})
	}
}

func rightClickShelfbox(ctx context.Context, tconn *chrome.TestConn, url string, wantAllowed bool) error {
	ui := uiauto.New(tconn)

	searchNode := nodewith.NameContaining("Search your device, apps, settings").First()

	// Select shelf box first time.
	if url == "www.example.com" {
		if err := ui.LeftClick(searchNode)(ctx); err != nil {
			return errors.Wrap(err, "failed finding shelf and clicking it: ")
		}
	}

	if err := ui.RightClick(searchNode)(ctx); err != nil {
		return errors.Wrap(err, "failed to right click shelf box: ")
	}

	err := clipboard.CheckGreyPasteNode(ctx, ui)
	if err != nil && !wantAllowed {
		return err
	}
	if err == nil && wantAllowed {
		return errors.New("Paste node found greyed, expected focusable")
	}

	err = clipboard.CheckClipboardBubble(ctx, ui, url)
	// Clipboard DLP bubble is never expected on right click.
	if err == nil {
		return errors.New("Notification found, expected none")
	}

	return nil
}

func pasteShelfbox(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, url string, wantAllowed bool) error {
	ui := uiauto.New(tconn)

	searchNode := nodewith.NameContaining("Search your device, apps, settings").First()
	if err := uiauto.Combine("Paste content in shelf box",
		ui.LeftClick(searchNode),
		keyboard.AccelAction("ctrl+V"))(ctx); err != nil {
		return errors.Wrap(err, "failed to paste content in shelf box: ")
	}

	err := clipboard.CheckClipboardBubble(ctx, ui, url)
	if err != nil && !wantAllowed {
		return err
	}

	if err == nil && wantAllowed {
		return errors.New("Notification found, expected none")
	}

	return nil
}
