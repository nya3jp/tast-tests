// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"time"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
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
		Fixture:      "fakeDMS",
	})
}

func DataLeakPreventionRulesListDragdrop(ctx context.Context, s *testing.State) {
	fakeDMS := s.FixtValue().(*fakedms.FakeDMS)

	// DLP policy with clipboard blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable drag and drop of confidential content in restricted destination",
				Description: "User should not be able to drag and drop confidential content in restricted destination",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
						"chromium.org",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListDestinations{
					Urls: []string{
						"google.com",
					},
				},
				Restrictions: []*policy.DataLeakPreventionRulesListRestrictions{
					{
						Class: "CLIPBOARD",
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
	if err := fakeDMS.WritePolicyBlob(pb); err != nil {
		s.Fatal("Failed to write policies to FakeDMS: ", err)
	}

	// Start a Chrome instance that will fetch policies from the FakeDMS.
	// Policies are only updated after Chrome startup.
	cr, err := chrome.New(ctx,
		chrome.FakeLogin(chrome.Creds{User: fixtures.Username, Pass: fixtures.Password}),
		chrome.DMSPolicy(fakeDMS.URL))
	if err != nil {
		s.Fatal("Chrome login failed: ", err)
	}
	defer cr.Close(ctx)

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
			name:        "Chromium",
			wantAllowed: false,
			url:         "www.chromium.org",
			content:     "The Chromium Projects",
		},
		{
			name:        "Company",
			wantAllowed: true,
			url:         "www.company.com",
			content:     "One Environment",
		},
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			defer faillog.DumpUITreeWithScreenshotOnError(ctx, s.OutDir(), s.HasError, cr, "ui_tree_"+param.name)
			conn, err := cr.NewConn(ctx, "https://www.google.com/")
			if err != nil {
				s.Fatal("Failed to open page: ", err)
			}
			defer conn.Close()

			if _, err = cr.NewConn(ctx, "https://"+param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err = ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
			}

			if err = keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err = keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			ui := uiauto.New(tconn)

			s.Log("Splitting windows")

			tab := nodewith.Role(role.Tab).Nth(1)
			start, err := ui.Location(ctx, tab)
			if err != nil {
				s.Fatal("Failed to get locaton for tab: ", err)
			}

			info, err := display.GetPrimaryInfo(ctx, tconn)
			if err != nil {
				s.Fatal("Failed to get the primary display info: ", err)
			}

			end := coords.NewPoint(info.WorkArea.Right()-1, info.WorkArea.CenterY())

			if err := uiauto.Combine("Split window",
				mouse.Drag(tconn, start.CenterPoint(), end, time.Second*2))(ctx); err != nil {
				s.Fatalf("Failed to verify content preview for: %s", err)
			}

			s.Log("Draging and dropping content")

			content := nodewith.Name(param.content).First()
			start, err = ui.Location(ctx, content)
			if err != nil {
				s.Fatal("Failed to get locaton for content: ", err)
			}

			search := "Google Search"
			searchTab := nodewith.Name(search).First()
			endLocation, err := ui.Location(ctx, searchTab)
			if err != nil {
				s.Fatal("Failed to get locaton for google search: ", err)
			}

			if err := uiauto.Combine("Drag and Drop",
				mouse.Drag(tconn, start.CenterPoint(), endLocation.CenterPoint(), time.Second*2))(ctx); err != nil {
				s.Fatalf("Failed to verify content preview for: %s", err)
			}

			s.Log("Checking notification")

			notfication := verifyNotfication(ctx, tconn, param.url)

			if !param.wantAllowed && notfication != nil {
				s.Fatal("Couldn't check for notification: ", err)
			}

			if param.wantAllowed && notfication == nil {
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

func verifyNotfication(ctx context.Context, tconn *chrome.TestConn, url string) error {

	ui := uiauto.New(tconn)
	bubbleView := nodewith.ClassName("ClipboardDlpBubble").Role(role.Window)
	bubbleClass := nodewith.ClassName("ClipboardBlockBubble").Ancestor(bubbleView)
	bubbleButton := nodewith.Name("Got it").Role(role.Button).Ancestor(bubbleClass)
	messageBlocked := "Pasting from " + url + " to this location is blocked by administrator policy"
	bubble := nodewith.Name(messageBlocked).Role(role.StaticText).Ancestor(bubbleClass)

	if err := uiauto.Combine("Bubble ",
		ui.WaitUntilExists(bubbleView),
		ui.WaitUntilExists(bubbleButton),
		ui.WaitUntilExists(bubbleClass),
		ui.WaitUntilExists(bubble))(ctx); err != nil {
		return errors.Wrap(err, "failed to check for printing windows existance: ")
	}

	return nil
}
