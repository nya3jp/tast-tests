// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"
	"strings"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListClipboard,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction by copy and paste",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

type pasteFunc func(context.Context) (string, error)

// getPastedData returns clipboard content.
func getPastedData(tconn *chrome.TestConn, format string) pasteFunc {
	return func(ctx context.Context) (string, error) {
		var result string
		if err := tconn.Call(ctx, &result, `
		  (format) => {
		    let result;
		    document.addEventListener('paste', (event) => {
		      result = event.clipboardData.getData(format);
		    }, {once: true});
		    if (!document.execCommand('paste')) {
			    throw new Error('Failed to execute paste');
		    }
		    return result;
		  }`, format,
		); err != nil {
			return "", err
		}
		return result, nil
	}
}

func DataLeakPreventionRulesListClipboard(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// DLP policy with clipboard blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content in restricted destination",
				Description: "User should not be able to copy and paste confidential content in restricted destination",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
						"company.com",
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
		wantAllowed bool
		url         string
	}{
		{
			name:        "Example",
			wantAllowed: false,
			url:         "www.example.com",
		},
		{
			name:        "Company",
			wantAllowed: false,
			url:         "www.company.com",
		},
		{
			name:        "Chromium",
			wantAllowed: true,
			url:         "www.chromium.org",
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

			if _, err = cr.NewConn(ctx, "https://www.google.com/"); err != nil {
				s.Error("Failed to open page: ", err)
			}

			// Google.com have property of autofocus for content to be pasted.
			if err := keyboard.Accel(ctx, "Ctrl+V"); err != nil {
				s.Fatal("Failed to press Ctrl+V to paste content: ", err)
			}

			// Verify Notification Bubble.
			notification := testNotification(ctx, tconn, param.url)

			if !param.wantAllowed && notification != nil {
				s.Fatal("Couldn't check for notification: ", notification)
			}

			pastedString, err := getPastedData(tconn, "text/plain")(ctx)
			if err != nil {
				s.Fatal("Failed to get clipboard content")
			}

			// Check Pasted content.
			pastedError := checkPastedContent(ctx, tconn, pastedString)

			if param.wantAllowed && pastedError != nil {
				s.Fatal("Couldn't check for pasted content: ", pastedError)
			}

			if (!param.wantAllowed && pastedError == nil) || (param.wantAllowed && notification == nil) {
				s.Fatal("Content pasted, expected restriction")
			}

		})
	}
}

func testNotification(ctx context.Context, tconn *chrome.TestConn, url string) error {

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
		return errors.Wrap(err, "failed to check for notification bubble existence: ")
	}

	return nil
}

func checkPastedContent(ctx context.Context, tconn *chrome.TestConn, content string) error {

	words := strings.Fields(content)
	content = strings.Join(words[:10], " ")

	ui := uiauto.New(tconn)
	contentNode := nodewith.NameContaining(content).Role(role.InlineTextBox).First()

	if err := uiauto.Combine("Pasted ",
		ui.WaitUntilExists(contentNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to check for pasted content: ")
	}

	return nil
}
