// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package quickanswers

import (
	"context"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/event"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/role"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func: Translation,
		Desc: "Test Quick Answers unit conversion feature",
		Contacts: []string{
			"updowndota@google.com",
			"croissant-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		VarDeps:      []string{"quickanswers.username", "quickanswers.password"},
	})
}

// Translation tests Quick Answers translation fearture.
func Translation(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx,
		chrome.GAIALogin(chrome.Creds{
			User: s.RequiredVar("quickanswers.username"),
			Pass: s.RequiredVar("quickanswers.password"),
		}),
		chrome.EnableFeatures("QuickAnswersV2"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setWhitelistedPref)`, "settings.quick_answers.enabled", true); err != nil {
		s.Fatal("Failed to enable Quick Answers: ", err)
	}

	// Open page with source units on it.
	conn, err := cr.NewConn(ctx, "https://google.com/search?q=翻译")
	if err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Wait for the source units to appear.
	units := nodewith.Name("翻译").Role(role.StaticText).First()
	if err := ui.WaitUntilExists(units)(ctx); err != nil {
		s.Fatal("Failed to wait for units to load: ", err)
	}

	// Select the units and setup watcher to wait for text selection event
	if err := ui.WaitForEvent(nodewith.Root(),
		event.TextSelectionChanged,
		ui.Select(units, 0, units, 2))(ctx); err != nil {
		s.Fatal("Failed to select units: ", err)
	}

	// Right click the selected units and ensure the Quick Answers UI shows up with the conversion result in pounds.
	quickAnswers := nodewith.ClassName("QuickAnswersView")
	unitConversionResult := nodewith.NameContaining("translate").ClassName("Label")
	if err := uiauto.Combine("Show context menu",
		ui.RightClick(units),
		ui.WaitUntilExists(quickAnswers),
		ui.WaitUntilExists(unitConversionResult))(ctx); err != nil {
		s.Fatal("Quick Answers result not showing up: ", err)
	}
}
