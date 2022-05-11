// Copyright 2018 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package example

import (
	"context"
	"time"

	"chromiumos/tast/local/a11y"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/local/chrome/uiauto/state"
	"chromiumos/tast/local/input"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Keyboard,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Demonstrates injecting keyboard events",
		Contacts:     []string{"ricardoq@chromium.org", "tast-owners@google.com"},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacrosPrimary",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}},
	})
}

func Keyboard(ctx context.Context, s *testing.State) {
	// Test Values
	const (
		html      = "<input id='text' type='text' label='example.Keyboard.TextBox' autofocus>"
		inputText = "Hello, world!"
	)

	// 1. Boilerplate setup + create tab with input form
	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}
	ui := uiauto.New(tconn).WithTimeout(10 * time.Second)
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	// Setup a browser before opening a tab.
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, s.Param().(browser.Type))
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(ctx)

	c, err := a11y.NewTabWithHTML(ctx, br, html)
	if err != nil {
		s.Fatal("Failed to open a new tab with HTML: ", err)
	}
	defer c.Close()

	// 2. Wait for focus on text box, then enter input text value
	s.Log("Waiting for focus")
	textbox := nodewith.NameContaining("label='example.Keyboard.TextBox'").Role(role.StaticText).Onscreen()

	if err := uiauto.Combine("Focus text box",
		ui.WaitUntilExists(textbox),
		// TODO(crbug.com/1291585): ui.FocusAndWait doesn't seem to work on Lacros. Timed out waiting for event.Focus to occur.
		// Since the input element has 'autofocus' attribute commenting the line below won't affect the test results.
		//ui.FocusAndWait(textbox),
	)(ctx); err != nil {
		s.Fatal("Failed to focus the text box: ", err)
	}

	s.Log("Finding and opening keyboard device")
	ew, err := input.Keyboard(ctx)
	if err != nil {
		s.Fatal("Failed to open keyboard device: ", err)
	}
	defer ew.Close()

	s.Logf("Injecting keyboard events for %q", inputText)
	if err = ew.Type(ctx, inputText); err != nil {
		s.Fatal("Failed to write events: ", err)
	}

	// 3. Assert inputted text matches expected value
	textboxWithContent := nodewith.State(state.Editable, true).Role(role.InlineTextBox).Name(inputText)
	if err := ui.WaitUntilExists(textboxWithContent)(ctx); err != nil {
		s.Fatal("Failed to verify text input: ", err)
	}

	const (
		pageText = "mittens"
		dataURL  = "data:text/plain," + pageText
		bodyExpr = "document.body.innerText"
	)
	s.Logf("Navigating to %q via omnibox", dataURL)
	if err := ew.Accel(ctx, "Ctrl+L"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}
	if err := ew.Type(ctx, dataURL+"\n"); err != nil {
		s.Fatal("Failed to write events: ", err)
	}
	mittensOutput := nodewith.Name(pageText).Role(role.InlineTextBox)
	if err := ui.WaitUntilExists(mittensOutput)(ctx); err != nil {
		s.Fatal("Failed to verify page text: ", err)
	}

	// Not all Chromebooks have the same layout for the function keys.
	layout, err := input.KeyboardTopRowLayout(ctx, ew)
	if err != nil {
		s.Fatal("Failed to get keyboard mapping: ", err)
	}

	key := layout.ZoomToggle
	// If the key is empty it means it is not mapped
	if key != "" {
		if err := ew.Accel(ctx, key); err != nil {
			s.Fatal("Failed to write events: ", err)
		}
	}
}
