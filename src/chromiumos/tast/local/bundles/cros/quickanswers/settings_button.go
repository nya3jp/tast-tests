// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quickanswers

import (
	"context"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/browser"
	"chromiumos/tast/local/chrome/browser/browserfixt"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         SettingsButton,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Test Quick Answers settings button",
		Contacts: []string{
			"updowndota@google.com",
			"croissant-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Params: []testing.Param{{
			Fixture: "chromeLoggedIn",
			Val:     browser.TypeAsh,
		}, {
			Name:              "lacros",
			Fixture:           "lacros",
			ExtraSoftwareDeps: []string{"lacros"},
			Val:               browser.TypeLacros,
		}}})
}

// SettingsButton tests Quick Answers settings button.
func SettingsButton(ctx context.Context, s *testing.State) {
	cleanupCtx := ctx
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr := s.FixtValue().(chrome.HasChrome).Chrome()

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setWhitelistedPref)`, "settings.quick_answers.enabled", true); err != nil {
		s.Fatal("Failed to enable Quick Answers: ", err)
	}

	// Setup a browser.
	bt := s.Param().(browser.Type)
	br, closeBrowser, err := browserfixt.SetUp(ctx, cr, bt)
	if err != nil {
		s.Fatal("Failed to open the browser: ", err)
	}
	defer closeBrowser(cleanupCtx)

	// Open page with source units on it.
	conn, err := br.NewConn(ctx, "https://google.com/search?q=50+kg")
	if err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	ui := uiauto.New(tconn)
	// Wait for the source units to appear.
	units := nodewith.Name("50 kg").Role(role.StaticText).First()
	if err := ui.WaitUntilExists(units)(ctx); err != nil {
		s.Fatal("Failed to wait for units to load: ", err)
	}

	// Select the units and setup watcher to wait for text selection event.
	if err := ui.WaitForEvent(nodewith.Root(),
		event.TextSelectionChanged,
		ui.Select(units, 0 /*startOffset*/, units, 5 /*endOffset*/))(ctx); err != nil {
		s.Fatal("Failed to select units: ", err)
	}

	// Right click the selected units and ensure the Quick Answers UI shows up
	// with the settings button and the conversion result in pounds.
	quickAnswers := nodewith.ClassName("QuickAnswersView")
	settingsButton := nodewith.ClassName("ImageButton").Name("Quick answers settings")
	unitConversionResult := nodewith.NameContaining("110.231").ClassName("QuickAnswersTextLabel")
	if err := uiauto.Combine("Show context menu",
		ui.RightClick(units),
		ui.WaitUntilExists(quickAnswers),
		ui.WaitUntilExists(settingsButton),
		ui.WaitUntilExists(unitConversionResult))(ctx); err != nil {
		s.Fatal("Quick Answers card not showing up: ", err)
	}

	// Click on the settings button and ensure the OS settings subpage show up.
	OsSettingsQuickAnswersToggleButton := nodewith.NameContaining("Quick answers").Role(role.ToggleButton)
	if err := uiauto.Combine("Click settings button",
		ui.LeftClick(settingsButton),
		ui.WaitUntilExists(OsSettingsQuickAnswersToggleButton))(ctx); err != nil {
		s.Fatal("Quick Answers settings subpage not showing up: ", err)
	}
}
