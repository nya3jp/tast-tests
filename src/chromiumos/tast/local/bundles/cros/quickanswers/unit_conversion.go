// Copyright 2021 The Chromium OS Authors. All rights reserved.
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
		Func: UnitConversion,
		Desc: "Test Quick Answers unit conversion feature",
		Contacts: []string{
			"updowndota@google.com",
			"croissant-eng@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// UnitConversion tests Quick Answers unit conversion fearture.
func UnitConversion(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(ctx, chrome.EnableFeatures("QuickAnswersV2"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setQuickAnswersEnabled)`, true); err != nil {
		s.Fatal("Failed to enable Quick Answers: ", err)
	}

	// Open page with source units on it.
	if _, err := cr.NewConn(ctx, "https://google.com/search?q=50+kg"); err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}

	// Wait for the source units to appear.
	units := nodewith.Name("50 kg").Role(role.StaticText).First()
	if err := ui.WaitUntilExists(units)(ctx); err != nil {
		s.Fatal("Failed to wait for units to load: ", err)
	}

	// Select the units and setup watcher to wait for text selection event
	if err := ui.WaitForEvent(nodewith.Root(),
		event.TextSelectionChanged,
		ui.Select(units, 0, units, 5))(ctx); err != nil {
		s.Fatal("Failed to select units: ", err)
	}

	// Right click the selected units and ensure the Quick Answers UI shows up with the conversion result in pounds.
	quickAnswers := nodewith.ClassName("QuickAnswersView")
	unitConversionResult := nodewith.NameContaining("110.231").ClassName("Label")
	if err := uiauto.Combine("Show context menu",
		ui.RightClick(units),
		ui.WaitUntilExists(quickAnswers),
		ui.WaitUntilExists(unitConversionResult))(ctx); err != nil {
		s.Fatal("Quick Answers result not showing up: ", err)
	}
}
