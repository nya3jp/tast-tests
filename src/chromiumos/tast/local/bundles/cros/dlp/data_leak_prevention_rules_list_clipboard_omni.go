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
	"chromiumos/tast/local/chrome/ash"
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
		Func: DataLeakPreventionRulesListClipboardOmni,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction with omni box",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

func DataLeakPreventionRulesListClipboardOmni(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// DLP policy with all clipboard blocked restriction.
	policyDLP := policy.GetAllClipboardDlpPolicy()

	// Update the policy blob.
	pb := fakedms.NewPolicyBlob()
	pb.AddPolicies(policyDLP)
	if err := fakeDMS.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
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

	s.Log("Waiting for chrome.clipboard API to become available")
	if err := tconn.WaitForExpr(ctx, "chrome.clipboard"); err != nil {
		s.Fatal("chrome.clipboard API unavailable: ", err)
	}

	for _, param := range []struct {
		name string
		url  string
	}{
		{
			name: "Example",
			url:  "www.example.com",
		},
		{
			name: "Company",
			url:  "www.company.com",
		},
		{
			name: "Chromium",
			url:  "www.chromium.org",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {
			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if _, err = cr.NewConn(ctx, "https://"+param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			s.Log("Right clicking omni box")

			err = rightClickOmnibox(ctx, tconn, param.url)

			if err != nil {
				s.Fatal("Failed to right click omni box: ", err)
			}

			s.Log("Pasting content in omni box")

			err = pasteOmnibox(ctx, tconn, keyboard, param.url)

			if err != nil {
				s.Fatal("Failed to paste content in omni box: ", err)
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

func rightClickOmnibox(ctx context.Context, tconn *chrome.TestConn, url string) error {
	ui := uiauto.New(tconn)

	addressBar := nodewith.Name("Address and search bar").First()

	if err := ui.RightClick(addressBar)(ctx); err != nil {
		return errors.Wrap(err, "failed to right click omni box: ")
	}

	err := clipboard.CheckPasteNode(ctx, ui)

	if err != nil {
		return err
	}

	err = clipboard.CheckClipboardBubble(ctx, ui, url)

	if err == nil {
		return errors.New("Notification found, expected none")
	}

	return nil
}

func pasteOmnibox(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, url string) error {
	ui := uiauto.New(tconn)

	addressBar := nodewith.Name("Address and search bar").First()

	if err := uiauto.Combine("Paste content in omni box",
		ui.WaitUntilExists(addressBar),
		ui.LeftClick(addressBar),
		keyboard.AccelAction("ctrl+V"))(ctx); err != nil {
		return errors.Wrap(err, "failed to paste content in omni box: ")
	}

	err := clipboard.CheckClipboardBubble(ctx, ui, url)

	return err
}
