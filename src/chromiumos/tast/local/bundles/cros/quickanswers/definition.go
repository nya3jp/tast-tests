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
		Func:         Definition,
		LacrosStatus: testing.LacrosVariantNeeded,
		Desc:         "Test Quick Answers definition feature",
		Contacts: []string{
			"updowndota@google.com",
			"croissant-eng@google.com",
			"chromeos-sw-engprod@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedIn",
	})
}

// Definition tests Quick Answers definition fearture.
func Definition(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	if err := tconn.Call(ctx, nil, `tast.promisify(chrome.autotestPrivate.setWhitelistedPref)`, "settings.quick_answers.enabled", true); err != nil {
		s.Fatal("Failed to enable Quick Answers: ", err)
	}

	// Open page with the query word on it.
	const queryWord = "icosahedron"
	conn, err := cr.NewConn(ctx, "https://google.com/search?q="+queryWord)
	if err != nil {
		s.Fatal("Failed to create new Chrome connection: ", err)
	}
	defer conn.Close()
	defer conn.CloseTarget(ctx)

	// Wait for the query word to appear.
	query := nodewith.Name(queryWord).Role(role.StaticText).First()
	if err := ui.WaitUntilExists(query)(ctx); err != nil {
		s.Fatal("Failed to wait for query to load: ", err)
	}

	// Select the word and setup watcher to wait for text selection event.
	if err := ui.WaitForEvent(nodewith.Root(),
		event.TextSelectionChanged,
		ui.Select(query, 0 /*startOffset*/, query, 2 /*endOffset*/))(ctx); err != nil {
		s.Fatal("Failed to select query: ", err)
	}

	// Right click the selected word and ensure the Quick Answers UI shows up with the definition result.
	quickAnswers := nodewith.ClassName("QuickAnswersView")
	definitionResult := nodewith.NameContaining("twenty plane faces").ClassName("Label")
	if err := uiauto.Combine("Show context menu",
		ui.RightClick(query),
		ui.WaitUntilExists(quickAnswers),
		ui.WaitUntilExists(definitionResult))(ctx); err != nil {
		s.Fatal("Quick Answers result not showing up: ", err)
	}

	// Dismiss the context menu and ensure the Quick Answers UI also dismiss.
	if err := uiauto.Combine("Dismiss context menu",
		ui.LeftClick(query),
		ui.WaitUntilGone(quickAnswers))(ctx); err != nil {
		s.Fatal("Quick Answers result not dismissed: ", err)
	}
}
