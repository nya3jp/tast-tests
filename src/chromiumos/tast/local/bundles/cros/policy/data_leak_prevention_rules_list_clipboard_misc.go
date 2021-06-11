// Copyright 2021 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package policy

import (
	"context"

	"chromiumos/tast/common/policy"
	"chromiumos/tast/common/policy/fakedms"
	"chromiumos/tast/errors"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListClipboardMisc,
		Desc: "Test behavior of DataLeakPreventionRulesList policy with clipboard blocked restriction in miscellaneous conditions",
		Contacts: []string{
			"vishal38785@gmail.com", // Test author
			"chromeos-dlp@google.com",
		},
		SoftwareDeps: []string{"chrome"},
		Attr:         []string{"group:mainline", "informational"},
		Fixture:      "chromePolicyLoggedIn",
	})
}

// pasteContent returns clipboard content.
func pasteContent(tconn *chrome.TestConn, format string) func(context.Context) (string, error) {
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

func DataLeakPreventionRulesListClipboardMisc(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*fixtures.FixtData).Chrome
	fakeDMS := s.FixtValue().(*fixtures.FixtData).FakeDMS

	// DLP policy with clipboard blocked restriction.
	policyDLP := []policy.Policy{&policy.DataLeakPreventionRulesList{
		Val: []*policy.DataLeakPreventionRulesListValue{
			{
				Name:        "Disable copy and paste of confidential content in any destination",
				Description: "User should not be able to copy and paste confidential content in any destination",
				Sources: &policy.DataLeakPreventionRulesListSources{
					Urls: []string{
						"example.com",
						"company.com",
						"chromium.org",
					},
				},
				Destinations: &policy.DataLeakPreventionRulesListDestinations{
					Urls: []string{
						"*",
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

			if err := ash.CloseNotifications(ctx, tconn); err != nil {
				s.Fatal("Failed to close notifications: ", err)
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

			s.Log("Right clicking shelf box")

			err := rightClickShelfbox(ctx, tconn, param.name)

			if err != nil {
				s.Fatal("Failed to right click shelf box: ", err)
			}

			s.Log("Pasting content in shelf box")

			err = pasteShelfbox(ctx, tconn, keyboard, param.url)

			if err != nil {
				s.Fatal("Failed to paste content in shelf box: ", err)
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

			s.Log("Checking copied content using extension")

			err = checkExtensionAccess(ctx, tconn, param.url)

			if err != nil {
				s.Fatal("Failed to check copied content: ", err)
			}

			s.Log("Opening files app")

			err = openFilesApp(ctx, tconn, param.url)

			if err != nil {
				s.Fatal("Failed to open filesapp: ", err)
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

func checkExtensionAccess(ctx context.Context, tconn *chrome.TestConn, url string) error {
	pastedString, err := pasteContent(tconn, "text/plain")(ctx)
	if err != nil {
		return errors.Wrap(err, "failed to get clipboard content: ")
	}

	if pastedString != "" {
		return errors.New("Extension able to access confidential content")
	}

	err = checkNotification(ctx, tconn, url)

	return err
}

func rightClickOmnibox(ctx context.Context, tconn *chrome.TestConn, url string) error {
	ui := uiauto.New(tconn)

	addressBar := nodewith.Name("Address and search bar").First()

	if err := uiauto.Combine("Right click omni box",
		ui.RightClick(addressBar))(ctx); err != nil {
		return errors.Wrap(err, "failed to right click omni box: ")
	}

	err := checkPasteNode(ctx, tconn)

	if err != nil {
		return err
	}

	err = checkNotification(ctx, tconn, url)

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

	err := checkNotification(ctx, tconn, url)

	return err
}

func openShelfbox(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	shelf := nodewith.Name("Launcher").ClassName("ash/HomeButton")

	if err := uiauto.Combine("Open shelf box",
		ui.LeftClick(shelf))(ctx); err != nil {
		return errors.Wrap(err, "failed to open shelf box: ")
	}

	return nil
}

func rightClickShelfbox(ctx context.Context, tconn *chrome.TestConn, name string) error {

	err := openShelfbox(ctx, tconn)

	if err != nil {
		return err
	}

	ui := uiauto.New(tconn)

	searchNode := nodewith.NameContaining("Search your device, apps, settings").First()

	// Select shelf box first time.
	if name == "Example" {
		if err := ui.LeftClick(searchNode)(ctx); err != nil {
			return errors.Wrap(err, "failed finding shelf and clicking it: ")
		}
	}

	if err := uiauto.Combine("Right click shelf box",
		ui.RightClick(searchNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to right click shelf box: ")
	}

	err = checkPasteNode(ctx, tconn)

	return err
}

func pasteShelfbox(ctx context.Context, tconn *chrome.TestConn, keyboard *input.KeyboardEventWriter, url string) error {

	err := openShelfbox(ctx, tconn)

	if err != nil {
		return err
	}

	ui := uiauto.New(tconn)

	searchNode := nodewith.NameContaining("Search your device, apps, settings").First()

	if err := uiauto.Combine("Paste content in shelf box",
		ui.LeftClick(searchNode),
		keyboard.AccelAction("ctrl+V"))(ctx); err != nil {
		return errors.Wrap(err, "failed to paste content in shelf box: ")
	}

	err = checkNotification(ctx, tconn, url)

	return err
}

func openFilesApp(ctx context.Context, tconn *chrome.TestConn, url string) error {

	// Open Files app
	filesApp, err := filesapp.Launch(ctx, tconn)
	if err != nil {
		return errors.Wrap(err, "failed to open files app: ")
	}

	err = checkNotification(ctx, tconn, url)

	if err == nil {
		return errors.New("Notification found, expected none")
	}

	filesApp.Close(ctx)

	return nil

}

func checkNotification(ctx context.Context, tconn *chrome.TestConn, url string) error {
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

func checkPasteNode(ctx context.Context, tconn *chrome.TestConn) error {
	ui := uiauto.New(tconn)

	pasteNode := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem)

	if err := uiauto.Combine("Check paste node greyed",
		ui.WaitUntilExists(pasteNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to check paste node greyed: ")
	}

	return nil
}
