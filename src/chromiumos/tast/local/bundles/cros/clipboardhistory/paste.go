// Copyright 2022 The ChromiumOS Authors
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

type pasteActionParams struct {
	item *nodewith.Finder
	ui   *uiauto.Context
	kbd  *input.KeyboardEventWriter
}

type pasteTestParams struct {
	testFn      func(pasteActionParams) uiauto.Action
	browserType browser.Type
}

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
			Name: "click_ash",
			Val: pasteTestParams{
				testFn:      pasteOnClick,
				browserType: browser.TypeAsh,
			},
			Fixture: "chromeLoggedIn",
		}, {
			Name: "click_lacros",
			Val: pasteTestParams{
				testFn:      pasteOnClick,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
		}, {
			Name: "enter_ash",
			Val: pasteTestParams{
				testFn:      pasteOnEnter,
				browserType: browser.TypeAsh,
			},
			Fixture: "chromeLoggedIn",
		}, {
			Name: "enter_lacros",
			Val: pasteTestParams{
				testFn:      pasteOnEnter,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
		}, {
			Name: "toggle_ash",
			Val: pasteTestParams{
				testFn:      pasteOnToggle,
				browserType: browser.TypeAsh,
			},
			Fixture: "chromeLoggedIn",
		}, {
			Name: "toggle_lacros",
			Val: pasteTestParams{
				testFn:      pasteOnToggle,
				browserType: browser.TypeLacros,
			},
			ExtraSoftwareDeps: []string{"lacros"},
			Fixture:           "lacros",
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
		html      = "<input id='text' type='text' label='textfield' autofocus>"
		inputText = "abc"
	)

	params := s.Param().(pasteTestParams)

	// It is fine not to use a new Chrome instance as long as we always add an
	// item to clipboard history and expect it to be the menu's top item.
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)
	defer faillog.SaveScreenshotOnError(ctx, cr, s.OutDir(), s.HasError)

	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, params.browserType)
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
		// The textfield should be empty initially.
		ui.WaitUntilExists(emptyTextbox),

		// The textfield auto-focuses, so we can begin typing once it exists.
		kbd.TypeAction(inputText),

		// The textfield should now contain the user's input.
		ui.WaitUntilExists(populatedTextbox),

		// Copy the user's input to the clipboard.
		kbd.AccelAction("Ctrl+A"),
		kbd.AccelAction("Ctrl+C"),
	)(ctx); err != nil {
		s.Fatal("Failed to populate clipboard history: ", err)
	}

	if err := uiauto.Combine("clear textfield",
		ui.LeftClick(populatedTextbox),
		kbd.AccelAction("Ctrl+A"),
		kbd.AccelAction("Backspace"),
		ui.WaitUntilExists(emptyTextbox),
	)(ctx); err != nil {
		s.Fatal("Failed to clear textfield: ", err)
	}

	// Open the clipboard history menu and paste into the now-empty textfield.
	if err := kbd.Accel(ctx, "Search+V"); err != nil {
		s.Fatal("Failed to launch clipboard history menu: ", err)
	}

	item := clipboardhistory.FindFirstTextItem()
	if err := uiauto.Combine("paste from clipboard history",
		// Make sure the clipboard history menu is pulled up and populated with the
		// previously-copied item.
		ui.WaitUntilExists(item),

		// Test one of the actions that pastes from clipboard history.
		params.testFn(pasteActionParams{item, ui, kbd}),

		// Make sure that once the clipboard history item is pasted, the menu is
		// closed and the textfield contains the item's text.
		ui.WaitUntilGone(item),
		ui.WaitUntilExists(populatedTextbox),
	)(ctx); err != nil {
		s.Fatal("Failed to paste from clipboard history: ", err)
	}
}

// pasteOnClick pastes `params.item` by clicking on it.
func pasteOnClick(params pasteActionParams) uiauto.Action {
	return params.ui.LeftClick(params.item)
}

// pasteOnEnter pastes `params.item` by pressing Enter.
func pasteOnEnter(params pasteActionParams) uiauto.Action {
	return params.kbd.AccelAction("Enter")
}

// pasteOnToggle pastes `params.item` by toggling the menu closed.
func pasteOnToggle(params pasteActionParams) uiauto.Action {
	return params.kbd.AccelAction("Search+V")
}
