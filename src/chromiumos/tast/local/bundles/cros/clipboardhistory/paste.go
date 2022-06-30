// Copyright 2022 The ChromiumOS Authors.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"time"

	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/clipboardhistory"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

type pasteAction func(*nodewith.Finder, *uiauto.Context, *input.KeyboardEventWriter) uiauto.Action

func init() {
	testing.AddTest(&testing.Test{
		Func:         Paste,
		LacrosStatus: testing.LacrosVariantUnneeded,
		Desc:         "Verifies different methods for pasting from clipboard history",
		Contacts: []string{
			"ckincaid@google.com",
			"multipaste@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// Paste verifies clipboard history paste behavior. It is expected that users
// can paste from the clipboard history menu by clicking on a history item,
// pressing enter with a history item focuses, or toggling the menu closed with
// a history item focused.
func Paste(ctx context.Context, s *testing.State) {
	// Set up browser window with text input field.
	const (
		html      = "<input id='text' type='text' label='textfield' autofocus>"
		inputText = "abc"
	)

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, browser.TypeAsh)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	c, err := a11y.NewTabWithHTML(ctx, br, html)
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	defer c.Close()

	kbd, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer kbd.Close()

	// Use the text input field to put an item in the clipboard history menu.
	emptyTextbox := nodewith.NameContaining("label='textfield'").Role(role.StaticText).Onscreen()
	populatedTextbox := nodewith.Name(inputText).Role(role.InlineTextBox)
	if err := uiauto.Combine("populate clipboard history",
		// The text field should be empty initially.
		ui.WaitUntilExists(emptyTextbox),

		// The text field auto-focuses, so we can begin typing once it exists.
		kbd.TypeAction(inputText),

		// The text field should now contain the user's input.
		ui.WaitUntilExists(populatedTextbox),

		// Copy the user's input to the clipboard.
		kbd.AccelAction("Ctrl+A"),
		kbd.AccelAction("Ctrl+C"),
	)(ctx); err != nil {
		s.Fatal("Failed to populate clipboard history: ", err)
	}

	// Test different ways of pasting from clipboard history.
	for _, tc := range []struct {
		name string
		fn   pasteAction
	}{
		{
			name: "click",
			fn:   pasteOnClick,
		},
		{
			name: "enter",
			fn:   pasteOnEnter,
		},
		{
			name: "toggle",
			fn:   pasteOnToggle,
		},
	} {
		if err := uiauto.Combine("clear textfield",
			ui.LeftClick(populatedTextbox),
			kbd.AccelAction("Ctrl+A"),
			kbd.AccelAction("Backspace"),
			ui.WaitUntilExists(emptyTextbox),
		)(ctx); err != nil {
			s.Fatal("Failed to clear textfield: ", err)
		}

		if err := kbd.Accel(ctx, "Search+V"); err != nil {
			s.Fatal("Failed to launch clipboard history menu: ", err)
		}

		menu := clipboardhistory.FindMenu()
		if err := uiauto.Combine("paste from clipboard history",
			// Make sure the clipboard history menu is pulled up.
			ui.WaitUntilExists(menu),

			// Test one of the actions that pastes from clipboard history.
			tc.fn(menu, ui, kbd),

			// Make sure that once the menu is closed, the text field contains text
			// pasted from clipboard history.
			ui.WaitUntilGone(menu),
			ui.WaitUntilExists(populatedTextbox),
		)(ctx); err != nil {
			s.Fatalf("Failed to paste from clipboard history on %q: %s", tc.name, err)
		}
	}
}

// pasteOnClick pastes from `menu` by clicking on it.
func pasteOnClick(menu *nodewith.Finder, ui *uiauto.Context, kbd *input.KeyboardEventWriter) uiauto.Action {
	return ui.LeftClick(menu)
}

// pasteOnEnter pastes from `menu` by pressing Enter.
func pasteOnEnter(menu *nodewith.Finder, ui *uiauto.Context, kbd *input.KeyboardEventWriter) uiauto.Action {
	return kbd.AccelAction("Enter")
}

// pasteOnToggle pastes from `menu` by toggling the menu closed.
func pasteOnToggle(menu *nodewith.Finder, ui *uiauto.Context, kbd *input.KeyboardEventWriter) uiauto.Action {
	return kbd.AccelAction("Search+V")
}
