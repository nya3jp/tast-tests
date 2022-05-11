// Copyright 2022 The Chromium OS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package calendar

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/ctxutil"
	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/quicksettings"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ButtonClicks,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the basic interacting with calendar view",
		Contacts: []string{
			"jiamingc@chromium.org",
			"cros-calendar@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		// Fixture:      "chromeLoggedIn",
	})
}

// ButtonClicks verifies that we can open, click buttons up/down, today, setting buttons correctly on the Calendar view.
func ButtonClicks(ctx context.Context, s *testing.State) {
	ctx, cancel := ctxutil.Shorten(ctx, 5*time.Second)
	defer cancel()

	cr, err := chrome.New(ctx,
		chrome.EnableFeatures("CalendarView"))
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	// cr := s.FixtValue().(*chrome.Chrome)
	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	s.Log("Start testing calendar view from quick settings")
	if err := quicksettings.Expand(ctx, tconn); err != nil {
		s.Fatal("Fail to open quick settings")
	}

	ui := uiauto.New(tconn)

	// Opening the calenar view from the quick setting's page and back to the main view several times.
	const iterations = 5
	for i := 0; i < iterations; i++ {
		s.Logf("Opening Calendar view (iteration %d of %d)", i+1, iterations)

		if err := ui.LeftClick(quicksettings.DateView)(ctx); err != nil {
			s.Fatal("Failed to click the DateView in quick settings bubble: ", err)
		}

		backButton := nodewith.Name("Previous menu").ClassName("IconButton")
		if err := ui.LeftClick(backButton)(ctx); err != nil {
			s.Fatal("Failed to click the BackButton in calendar view bubble: ", err)
		}

	}

	if err := ui.LeftClick(quicksettings.DateView)(ctx); err != nil {
		s.Fatal("Failed to click the DateView in quick settings page: ", err)
	}

	Year := strconv.Itoa(time.Now().Year())
	todyYearLabel := nodewith.Name(Year).ClassName("Label").Onscreen()
	if err := ui.WaitUntilExists(todyYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after opening calendar view: ", err)
	}

	upButton := nodewith.Name("Show previous month").ClassName("IconButton")
	if err := ui.LeftClick(upButton)(ctx); err != nil {
		s.Fatal("Failed to click the up button in calendar view bubble: ", err)
	}

	// Wait for scrolling animation to finish.
	if err := testing.Sleep(ctx, 600*time.Millisecond); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	curentYear := strconv.Itoa(time.Now().Year())
	currentYearLabel := nodewith.Name(curentYear).ClassName("Label").Onscreen()
	if err := ui.WaitUntilExists(currentYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after clicking on up button: ", err)
	}

	// Clicking the up button for 12 times should go to the previous year.
	const bttonIterations = 12
	previousYear := strconv.Itoa(time.Now().Year() - 1)
	previousYearLabel := nodewith.Name(previousYear).ClassName("Label").Onscreen()
	for i := 0; i < bttonIterations; i++ {
		if err := ui.LeftClick(upButton)(ctx); err != nil {
			s.Fatal("Failed to click the up button in calendar view bubble: ", err)
		}
	}

	// Wait for scrolling animation to finish.
	if err := testing.Sleep(ctx, 600*time.Millisecond); err != nil {
		s.Fatal("Failed to wait: ", err)
	}
	if err := ui.WaitUntilExists(previousYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after clicking on up button: ", err)
	}

	// Clicking on today button should go back to today's momth.
	todayButton := nodewith.NameContaining("Today").ClassName("PillButton")
	if err := ui.LeftClick(todayButton)(ctx); err != nil {
		s.Fatal("Failed to click the today button in calendar view bubble: ", err)
	}

	// Wait for scrolling animation to finish.
	if err := testing.Sleep(ctx, 600*time.Millisecond); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	if err := ui.WaitUntilExists(todyYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after clicking on today button: ", err)
	}

	// Clicking the down button for 12 times should go to the next year.
	nextyear := strconv.Itoa(time.Now().Year() + 1)
	nextYearLabel := nodewith.Name(nextyear).ClassName("Label").Onscreen()
	downButton := nodewith.Name("Show next month").ClassName("IconButton")
	for i := 0; i < bttonIterations; i++ {
		if err := ui.LeftClick(downButton)(ctx); err != nil {
			s.Fatal("Failed to click the down button in calendar view bubble: ", err)
		}
	}

	// Wait for scrolling animation to finish.
	if err := testing.Sleep(ctx, 600*time.Millisecond); err != nil {
		s.Fatal("Failed to wait: ", err)
	}

	if err := ui.WaitUntilExists(nextYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after clicking on down button: ", err)
	}

	settingButton := nodewith.Name("Date and time settings").ClassName("IconButton")
	if err := ui.LeftClick(settingButton)(ctx); err != nil {
		s.Fatal("Failed to click the setting button in calendar view bubble: ", err)
	}

	// Check if the DateTime setting page within the OS Settings was opened.
	matcher := chrome.MatchTargetURL("chrome://os-settings/dateTime")
	conn, err := cr.NewConnForTarget(ctx, matcher)
	if err != nil {
		s.Fatal("Failed to open the date and time settings: ", err)
	}
	defer conn.Close()

}
