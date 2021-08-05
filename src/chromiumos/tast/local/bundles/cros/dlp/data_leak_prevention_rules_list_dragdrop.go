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
	"chromiumos/tast/local/chrome/cdputil"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
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
	policyDLP := policy.StandardDLPPolicyForClipboard()

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

			if _, err = cr.NewConn(ctx, "https://"+param.url, cdputil.WithNewWindow()); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err = keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err = keyboard.Accel(ctx, "Alt+]"); err != nil {
				s.Fatal("Failed to press Alt+] to snap window to right: ", err)
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
		return errors.Wrap(err, "failed to get locaton for content: ")
	}

	search := "Google Search"
	searchTab := nodewith.Name(search).Role(role.InlineTextBox).First()
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
