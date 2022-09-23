// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package clipboardhistory

import (
	"context"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
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

type pasteTestParams struct {
	testFn      func(*uiauto.Context, *input.KeyboardEventWriter, *nodewith.Finder) uiauto.Action
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
		text = "abc"
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

	conn, err := br.NewConn(ctx, "")
	if err != nil {
		s.Fatal("Failed to connect to chrome: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	kb, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer kb.Close()

	if err := ash.SetClipboard(ctx, tconn, text); err != nil {
		s.Fatalf("Failed to add %q to clipboard history: %v", text, err)
	}

	rootView := nodewith.NameStartingWith("about:blank").HasClass("BrowserRootView")
	searchbox := nodewith.Role(role.TextField).Name("Address and search bar").Ancestor(rootView)
	if err := clipboardhistory.PasteAndVerify(ui, kb, searchbox /*contextMenu=*/, false, params.testFn, text)(ctx); err != nil {
		s.Fatal("Failed to paste from clipboard history: ", err)
	}
}

// pasteOnClick pastes `item` by clicking on it.
func pasteOnClick(ui *uiauto.Context, kb *input.KeyboardEventWriter, item *nodewith.Finder) uiauto.Action {
	return ui.LeftClick(item.Focused())
}

// pasteOnEnter pastes `item` by pressing Enter.
func pasteOnEnter(ui *uiauto.Context, kb *input.KeyboardEventWriter, item *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("paste by pressing enter with item selected",
		ui.WaitUntilExists(item.Focused()),
		kb.AccelAction("Enter"),
	)
}

// pasteOnToggle pastes `item` by toggling the menu closed.
func pasteOnToggle(ui *uiauto.Context, kb *input.KeyboardEventWriter, item *nodewith.Finder) uiauto.Action {
	return uiauto.Combine("paste by toggling clipboard history with item selected",
		ui.WaitUntilExists(item.Focused()),
		kb.AccelAction("Search+V"),
	)
}
