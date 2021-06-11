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
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/filesapp"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/crostini/faillog"
	"chromiumos/tast/local/input"
	"chromiumos/tast/local/policyutil"
	"chromiumos/tast/local/policyutil/fixtures"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: DataLeakPreventionRulesListClipboardMisc,
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

func DataLeakPreventionRulesListClipboardMisc(ctx context.Context, s *testing.State) {
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
		name        string
		wantAllowed bool
		url         string
	}{
		{
			name:        "Example",
			wantAllowed: false,
			url:         "www.example.com",
		},
		// {
		// 	name:        "Company",
		// 	wantAllowed: false,
		// 	url:         "www.company.com",
		// },
		// {
		// 	name:        "Chromium",
		// 	wantAllowed: true,
		// 	url:         "www.chromium.org",
		// },
	} {
		s.Run(ctx, param.name, func(ctx context.Context, s *testing.State) {

			if _, err = cr.NewConn(ctx, "https://"+param.url); err != nil {
				s.Error("Failed to open page: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+A"); err != nil {
				s.Fatal("Failed to press Ctrl+A to select all content: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+C"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+T"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			s.Log("Right click shelf box")

			err := rightClickShelfbox(ctx, tconn)

			if err != nil {
				s.Fatal("Failed to right click shelf box: ", err)
			}

			s.Log("Pasting content in shelf box")

			err = pasteShelfbox(ctx, tconn, keyboard, param.url)

			if err != nil {
				s.Fatal("Failed to paste in shelf box: ", err)
			}

			s.Log("Right click omni box")

			err = rightClickOmnibox(ctx, tconn, param.url)

			if err != nil {
				s.Fatal("Failed to omni: ", err)
			}

			s.Log("Pasting content in omni box")

			err = pasteOmnibox(ctx, tconn, keyboard, param.url)

			if err != nil {
				s.Fatal("Failed to omni: ", err)
			}

			if err := keyboard.Accel(ctx, "Ctrl+T"); err != nil {
				s.Fatal("Failed to press Ctrl+C to copy content: ", err)
			}

			s.Log("Opening files app")

			err = openFilesApp(ctx, tconn, param.url)

			if err != nil {
				s.Fatal("Failed to filesapp: ", err)
			}

			faillog.DumpUITreeAndScreenshot(ctx, tconn, "resize_backup_r", "e")
		})
	}
}

func rightClickOmnibox(ctx context.Context, tconn *chrome.TestConn, url string) error {
	ui := uiauto.New(tconn)

	// node id=179 role=textField state={"editable":true,"focusable":true} parentID=383 childIds=[] name=Address and search bar className=OmniboxViewViews

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

	shelf := nodewith.Name("Launcher").First()

	if err := uiauto.Combine("Open shelf box",
		ui.LeftClick(shelf))(ctx); err != nil {
		return errors.Wrap(err, "failed to open shelf box: ")
	}

	return nil
}

func rightClickShelfbox(ctx context.Context, tconn *chrome.TestConn) error {

	err := openShelfbox(ctx, tconn)

	if err != nil {
		return err
	}

	ui := uiauto.New(tconn)

	searchNode := nodewith.NameContaining("Search your device, apps, settings").First()

	if err := uiauto.Combine("Right click shelf box",
		ui.WaitUntilExists(searchNode),
		ui.LeftClick(searchNode),
		ui.RightClick(searchNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to right click shelf box: ")
	}

	// faillog.DumpUITreeAndScreenshot(ctx, tconn, "resize_backup_r", "e")

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
		ui.WaitUntilExists(searchNode),
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
	defer filesApp.Close(ctx)

	err = checkNotification(ctx, tconn, url)

	if err == nil {
		return errors.New("Notification found, expected none")
	}

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

	pasteNodeTrue := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem).State(state.Focusable, true)
	pasteNode := nodewith.Name("Paste Ctrl+V").Role(role.MenuItem)

	if err := uiauto.Combine("Check paste node not greyed",
		ui.WaitUntilExists(pasteNodeTrue))(ctx); err == nil {
		return errors.Wrap(err, "failed to check paste node not greyed: ")
	}

	if err := uiauto.Combine("Check paste node greyed",
		ui.WaitUntilExists(pasteNode))(ctx); err != nil {
		return errors.Wrap(err, "failed to check paste node greyed: ")
	}

	return nil
}

// func testNotification(ctx context.Context, tconn *chrome.TestConn, url string) error {

// 	ui := uiauto.New(tconn)
// 	bubbleView := nodewith.ClassName("ClipboardDlpBubble").Role(role.Window)
// 	bubbleClass := nodewith.ClassName("ClipboardBlockBubble").Ancestor(bubbleView)
// 	bubbleButton := nodewith.Name("Got it").Role(role.Button).Ancestor(bubbleClass)
// 	messageBlocked := "Pasting from " + url + " to this location is blocked by administrator policy"
// 	bubble := nodewith.Name(messageBlocked).Role(role.StaticText).Ancestor(bubbleClass)

// 	if err := uiauto.Combine("Bubble ",                        node id=105 role=toolbar state={} parentID=106 childIds=[115,119,113] name=Shelf className=ShelfView
// 		ui.WaitUntilExists(bubbleView),
// 		ui.WaitUntilExists(bubbleButton),
// 		ui.WaitUntilExists(bubbleClass),
// 		ui.WaitUntilExists(bubble))(ctx); err != nil {
// 		return errors.Wrap(err, "failed to check for notification bubble existence: ")
// 	}

// 	return nil
// }
// Ids=[] name=Emoji Search+Shift+Space className=MenuItemView
//                         node id=826 role=splitter state={} parentID=824 childIds=[] className=MenuSeparator
//                         node id=811 role=menuItem state={} parentID=824 childIds=[] name=Undo Ctrl+Z className=MenuItemView
//                         node id=827 role=splitter state={} parentID=824 childIds=[] className=MenuSeparator
//                         node id=812 role=menuItem state={} parentID=824 childIds=[] name=Cut Ctrl+X className=MenuItemView
//                         node id=813 role=menuItem state={} parentID=824 childIds=[] name=Copy Ctrl+C className=MenuItemView
//                         node id=814 role=menuItem state={} parentID=824 childIds=[] name=Paste Ctrl+V className=MenuItemView
//                         node id=828 role=menuItem state={"focusable":true} parentID=824 childIds=[] name=Clipboard Search+V This is a new feature className=MenuItemView
//                         node id=815 role=menuItem state={} parentID=824 childIds=[] name=Delete className=MenuItemView

// node id=131 role=button state={"focusable":true} parentID=474 childIds=[] name=Minimize className=FrameCaptionButton
// node id=135 role=button state={"focusable":true} parentID=474 childIds=[] name=Close className=FrameCaptionButton
// node id=825 role=window state={} parentID=451 childIds=[821] className=ClipboardDlpBubble
// node id=821 role=window state={} parentID=825 childIds=[822] className=Widget
// node id=822 role=window state={"focused":true} parentID=821 childIds=[823] className=RootView
// node id=823 role=unknown state={} parentID=822 childIds=[827,818] className=ClipboardBlockBubble
// node id=827 role=staticText state={} parentID=823 childIds=[] name=Pasting from www.example.com to this location is blocked by administrator policy className=Label

// node id=818 role=button state={"focusable":true} parentID=823 childIds=[830] name=Got it className=Button
// node id=830 role=staticText state={} parentID=818 childIds=[] name=Got it className=LabelButtonLabel

// if err := uiauto.Combine("Right click shelf box",
// 		ui.WaitUntilExists(searchNode),
// 		ui.LeftClick(searchNode),
// 		keyboard.AccelAction("ctrl+V"))(ctx); err != nil {
// 		return errors.Wrap(err, "failed to show map option: ")
// 	}

//                         node id=135 role=button state={"focusable":true} parentID=474 childIds=[] name=Close className=FrameCaptionButton
//             node id=825 role=window state={} parentID=451 childIds=[821] className=ClipboardDlpBubble
//               node id=821 role=window state={} parentID=825 childIds=[822] className=Widget
//                 node id=822 role=window state={"focused":true} parentID=821 childIds=[823] className=RootView
//                   node id=823 role=unknown state={} parentID=822 childIds=[827,818] className=ClipboardBlockBubble
//                     node id=827 role=staticText state={} parentID=823 childIds=[] name=Pasting from www.example.com to this location is blocked by administrator policy className=Label
//                     node id=818 role=button state={"focusable":true} parentID=823 childIds=[830] name=Got it className=Button
//                       node id=830 role=staticText state={} parentID=818 childIds=[] name=Got it className=LabelButtonLabel
//           node id=452 role=window state={"invisible":true} parentID=437 childIds=[] className=Desk_Container_B
