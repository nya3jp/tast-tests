// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"

	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/chrome/clipboardhistory"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Paste,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Verifies different methods for pasting from clipboard history",
		Contacts: []string{
			"ckincaid@google.com",
			"multipaste@google.com",
			"chromeos-sw-engprod@google.com",
			"cros-system-ui-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Name:    "click_ash",
			Val:     pasteOnClick,
			Fixture: "clipboardHistory",
		}, {
			Name:              "click_lacros",
			Val:               pasteOnClick,
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "clipboardHistoryLacros",
		}, {
			Name:    "enter_ash",
			Val:     pasteOnEnter,
			Fixture: "clipboardHistory",
		}, {
			Name:              "enter_lacros",
			Val:               pasteOnEnter,
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "clipboardHistoryLacros",
		}, {
			Name:    "toggle_ash",
			Val:     pasteOnToggle,
			Fixture: "clipboardHistory",
		}, {
			Name:              "toggle_lacros",
			Val:               pasteOnToggle,
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "clipboardHistoryLacros",
		}},
	})
}

// Paste verifies clipboard history paste behavior. It is expected that users
// can paste from the clipboard history menu by clicking on a history item,
// pressing enter with a history item focuses, or toggling the menu closed with
// a history item focused.
func Paste(ctx context.Context, s *testing.State) {
	// Set up a browser window with a text input field.
	const (
		html = "<input id='text' type='text' label='textfield' autofocus>"
		text = "abc"
	)

	f := s.FixtValue().(*clipboardhistory.FixtData)
	ui := f.UI
	kb := f.Keyboard

	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, f.TestConn)
	defer faillog.SaveScreenshotOnError(ctx, f.Chrome, s.OutDir(), s.HasError)

	c, err := a11y.NewTabWithHTML(ctx, f.Browser, html)
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	defer c.Close()

	// Use the text input field to put an item in the clipboard history menu.
	// TODO utils pre-factor should hopefully fix this
	emptyTextbox := nodewith.NameContaining("label='textfield'").Role(role.StaticText).Onscreen()
	populatedTextbox := nodewith.Name(text).Role(role.InlineTextBox)
	if err := uiauto.Combine("populate clipboard history",
		// The textfield should be empty initially.
		ui.WaitUntilExists(emptyTextbox),

		// The textfield auto-focuses, so we can begin typing once it exists.
		kb.TypeAction(text),

		// The textfield should now contain the user's input.
		ui.WaitUntilExists(populatedTextbox),

		// Copy the user's input to the clipboard.
		kb.AccelAction("Ctrl+A"),
		kb.AccelAction("Ctrl+C"),
	)(ctx); err != nil {
		s.Fatal("Failed to populate clipboard history: ", err)
	}

	if err := uiauto.Combine("clear textfield",
		ui.LeftClick(populatedTextbox),
		kb.AccelAction("Ctrl+A"),
		kb.AccelAction("Backspace"),
		ui.WaitUntilExists(emptyTextbox),
	)(ctx); err != nil {
		s.Fatal("Failed to clear textfield: ", err)
	}

	// Open the clipboard history menu and paste into the now-empty textfield.
	if err := kb.Accel(ctx, "Search+V"); err != nil {
		s.Fatal("Failed to launch clipboard history menu: ", err)
	}

	pasteFn := s.Param().(func(*clipboardhistory.FixtData, *nodewith.Finder) uiauto.Action)
	item := clipboardhistory.FindFirstTextItem()
	if err := uiauto.Combine("paste from clipboard history",
		// Make sure the clipboard history menu is pulled up and populated with the
		// previously-copied item.
		ui.WaitUntilExists(item),

		// Test one of the actions that pastes from clipboard history.
		pasteFn(f, item),

		// Make sure that once the clipboard history item is pasted, the menu is
		// closed and the textfield contains the item's text.
		ui.WaitUntilGone(item),
		ui.WaitUntilExists(populatedTextbox),
	)(ctx); err != nil {
		s.Fatal("Failed to paste from clipboard history: ", err)
	}
}

// pasteOnClick pastes `item` by clicking on it.
func pasteOnClick(f *clipboardhistory.FixtData, item *nodewith.Finder) uiauto.Action {
	return f.UI.LeftClick(item)
}

// pasteOnEnter pastes `item` by pressing Enter.
func pasteOnEnter(f *clipboardhistory.FixtData, item *nodewith.Finder) uiauto.Action {
	return f.Keyboard.AccelAction("Enter")
}

// pasteOnToggle pastes `item` by toggling the menu closed.
func pasteOnToggle(f *clipboardhistory.FixtData, item *nodewith.Finder) uiauto.Action {
	return f.Keyboard.AccelAction("Search+V")
}
