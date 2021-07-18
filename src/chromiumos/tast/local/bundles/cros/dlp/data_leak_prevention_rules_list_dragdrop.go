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
	"chromiumos/tast/local/chrome/display"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListDragdrop,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction by drag and drop",
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
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// DLP policy with clipboard blocked restriction.
	policyDLP := policy.GetStandardClipboardDlpPolicy()

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

	for _, param := range []struct {
		name        string
		wantAllowed bool
		url         string
		content     string
	}{
		{
			name:        "Example",
			wantAllowed: false,
			url:         "www.example.com",
			content:     "Example Domain",
		},
		{
			name:        "Company",
			wantAllowed: false,
			url:         "www.company.com",
			content:     "One Environment",
		},
		{
			name:        "Chromium",
			wantAllowed: true,
			url:         "www.chromium.org",
			content:     "The Chromium Projects",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)

			if _, err = cr.NewConn(ctx, "https://www.google.com/"); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if _, err = cr.NewConn(ctx, "https://"+param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err = keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			s.Log("Splitting windows")

			if err := splitWindows(ctx, tconn); err != nil {
				s.Fatal("Failed to split the windows: ", err)
			}

			s.Log("Draging and dropping content")

			if err := dragDrop(ctx, tconn, param.content); err != nil {
				s.Fatal("Failed to drag drop content: ", err)
			}

			s.Log("Checking notification")

			ui := uiauto.New(tconn)

			notification := clipboard.CheckClipboardBubble(ctx, ui, param.url)

			if !param.wantAllowed && notification != nil {
				s.Fatal("Couldn't check for notification: ", notification)
			}

			if param.wantAllowed && notification == nil {
				s.Fatal("Content pasted, expected restriction")
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

func dragDrop(ctx context.Context, tconn *chrome.TestConn, content string) error {

	ui := uiauto.New(tconn)

	contentNode := nodewith.Name(content).First()
	start, err := ui.Location(ctx, contentNode)
	if err != nil {
		return errors.Wrap(err, "failed to get locaton for content: ")
	}

	search := "Google Search"
	searchTab := nodewith.Name(search).First()
	endLocation, err := ui.Location(ctx, searchTab)
	if err != nil {
		return errors.Wrap(err, "failed to get locaton for google search: ")
	}

	if err := uiauto.Combine("Drag and Drop",
		mouse.Drag(tconn, start.CenterPoint(), endLocation.CenterPoint(), time.Second*2))(ctx); err != nil {
		return errors.Wrap(err, "failed to verify content preview for: ")
	}
	return nil
}

func splitWindows(ctx context.Context, tconn *chrome.TestConn) error {

	ui := uiauto.New(tconn)

	tab := nodewith.Role(role.Tab).Nth(1)
	start, err := ui.Location(ctx, tab)
	if err != nil {
		return errors.Wrap(err, "failed to get locaton for tab: ")
	}

	info, err := display.GetPrimaryInfo(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to get the primary display info: ")
	}

	end := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

	if err := uiauto.Combine("Split window",
		mouse.Drag(tconn, start.CenterPoint(), end, time.Second*2))(ctx); err != nil {
		return errors.Wrap(err, "failed to verify content preview for: ")
	}

	return nil
}
