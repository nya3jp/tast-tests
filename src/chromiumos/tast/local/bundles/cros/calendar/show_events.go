// Copyright 2022 The ChromiumOS Authors. All rights reserved.
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package calendar

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ShowEvents,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the event list on the calendar view",
		Contacts: []string{
			"jiamingc@chromium.org",
			"cros-calendar@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithCalendarView",
	})
}

// ShowEvents verifies that we can show the calendar event list view correctly.
func ShowEvents(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	ui := uiauto.New(tconn)

	s.Log("Start testing calendar view from date tray")
	dateTray := nodewith.HasClass("DateTray")

	// Comparing the time before and after opening the calendar view just in case this test is run at the very end of a year, e.g. Dec 31 23:59:59.
	beforeOpeningCalendarYear := time.Now().Year()

	// Open calendar view.
	if err := ui.DoDefault(dateTray)(ctx); err != nil {
		s.Fatal("Failed to click the date tray: ", err)
	}

	calendarView := nodewith.HasClass("CalendarView")
	mainHeaderTriView := nodewith.HasClass("TriView").Ancestor(calendarView).Nth(0)
	mainHeaderContainer := nodewith.HasClass("View").Ancestor(mainHeaderTriView).Nth(1)
	mainHeader := nodewith.Name("Calendar").HasClass("Label").Ancestor(mainHeaderContainer)

	if err := ui.WaitUntilExists(mainHeader)(ctx); err != nil {
		s.Fatal("Failed to find calendar main label after opening calendar view: ", err)
	}

	// For some corner cases, if it cannot find the year label with the time before clicking on the date tray, it should find the year label with the time after the calendar view is open.
	// E.g. before opening it's Dec 31 23:59:59 2022, and after openting it's Jan 1 00:00 2023.
	yearInt := beforeOpeningCalendarYear
	beforeOpeningCalendarYearLabel := nodewith.Name(strconv.Itoa(beforeOpeningCalendarYear)).HasClass("Label").Onscreen()
	if found, err := ui.IsNodeFound(ctx, beforeOpeningCalendarYearLabel); err != nil {
		s.Fatal("Failed to check beforeOpeningCalendarYearLabel after clicking on the date tray: ", err)
	} else if found != true {
		yearInt = time.Now().Year()
	}

	// Opening the calendar view should show today's year label.
	year := strconv.Itoa(yearInt)
	todayYearLabel := nodewith.Name(year).HasClass("Label").Onscreen()
	if err := ui.WaitUntilExists(todayYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after opening calendar view: ", err)
	}

	// Click on a Monday's cell to show the event list view.
	scrollView := nodewith.HasClass("ScrollView").Ancestor(calendarView).Nth(0)
	scrollViewport := nodewith.HasClass("ScrollView::Viewport").Ancestor(scrollView).Nth(0)
	contentView := nodewith.HasClass("View").Ancestor(scrollViewport).Nth(0)
	currentMonthView := nodewith.HasClass("View").Ancestor(contentView).Nth(3)
	firstMondayDateCell := nodewith.HasClass("CalendarDateCellView").Ancestor(currentMonthView).Nth(1)
	firstTuesdayDateCell := nodewith.HasClass("CalendarDateCellView").Ancestor(currentMonthView).Nth(2)
	scrollViewBounds, err := ui.Location(ctx, scrollView)
	if err != nil {
		s.Fatal("Failed to find calendar scroll view bounds: ", err)
	}
	firstMondayDateCellBounds, err := ui.Location(ctx, firstMondayDateCell)
	if err != nil {
		s.Fatal("Failed to find calendar first Monday cell bounds: ", err)
	}

	// TODO(b/234673735): Should click on the finder directly after this bug is fixed.
	// Currently the vertical location of the cells fetched from |ui.Location| are not correct.
	// It returns the same number for the |Top| of all the cells, which is the same number as the |Top| of the scroll view.
	// This might because of the scroll view is nested in some other views.
	// Here a small amount (5) of pixel is added to the top of the scroll view each time in the loop to find the first available Monday cell with events.
	const findCellTimes = 20
	cellPositionY := 0
	eventListView := nodewith.HasClass("CalendarEventListView").Ancestor(calendarView)
	eventCloseButtonView := nodewith.HasClass("View").Ancestor(eventListView).Nth(0)
	for i := 0; i < findCellTimes; i++ {
		s.Logf("Moving towards the first Monday cell (iteration %d of %d)", i+1, findCellTimes)
		cellPositionY += 5
		firstMondayDateCellPt := coords.NewPoint(firstMondayDateCellBounds.CenterX(), scrollViewBounds.Top+cellPositionY)
		if err := mouse.Click(tconn, firstMondayDateCellPt, mouse.LeftButton)(ctx); err != nil {
			s.Fatal("Failed to click the first Monday date cell: ", err)
		}
		if found, err := ui.IsNodeFound(ctx, eventCloseButtonView); err != nil {
			s.Fatal("Failed to check event list view close button while finding the first Monday cell: ", err)
		} else if found == true {
			break
		}
	}

	// Show the 3 events on Monday's event list view.
	eventScrollView := nodewith.HasClass("ScrollView").Ancestor(eventListView).Nth(0)
	eventScrollViewport := nodewith.HasClass("ScrollView::Viewport").Ancestor(eventScrollView).Nth(0)
	eventContentView := nodewith.HasClass("View").Ancestor(eventScrollViewport).Nth(0)
	firstEventView := nodewith.HasClass("CalendarEventListItemView").Ancestor(eventContentView).Nth(0)
	secondEventView := nodewith.HasClass("CalendarEventListItemView").Ancestor(eventContentView).Nth(1)
	thirdEventView := nodewith.HasClass("CalendarEventListItemView").Ancestor(eventContentView).Nth(2)
	if err := ui.WaitUntilExists(eventCloseButtonView)(ctx); err != nil {
		s.Fatal("Failed to find close button after clicking on the first Monday date cell: ", err)
	}
	if err := ui.WaitUntilExists(firstEventView)(ctx); err != nil {
		s.Fatal("Failed to find the first event entry after clicking on the first Monday date cell: ", err)
	}
	if err := ui.WaitUntilExists(secondEventView)(ctx); err != nil {
		s.Fatal("Failed to find the second evnet entry after clicking on the first Monday date cell: ", err)
	}
	if err := ui.WaitUntilExists(thirdEventView)(ctx); err != nil {
		s.Fatal("Failed to find the third evnet entry after clicking on the first Monday date cell: ", err)
	}
	firstEventLabel := nodewith.NameContaining("Monday morning").HasClass("Label")
	if err := ui.WaitUntilExists(firstEventLabel)(ctx); err != nil {
		s.Fatal("Failed to find the first event label after clicking on the first Monday date cell: ", err)
	}
	secondEventLabel := nodewith.NameContaining("Monday second").HasClass("Label")
	if err := ui.WaitUntilExists(secondEventLabel)(ctx); err != nil {
		s.Fatal("Failed to find the second event label after clicking on the first Monday date cell: ", err)
	}
	thirdEventLabel := nodewith.NameContaining("Monday third").HasClass("Label")
	if err := ui.WaitUntilExists(thirdEventLabel)(ctx); err != nil {
		s.Fatal("Failed to find the third event label after clicking on the first Monday date cell: ", err)
	}

	// Close event list view.
	if err := ui.LeftClick(eventCloseButtonView)(ctx); err != nil {
		s.Fatal("Failed to click the close button in calendar event list view after open Monday's event list: ", err)
	}

	// Click on a deate with no events.
	firstTuesdayDateCellBounds, err := ui.Location(ctx, firstTuesdayDateCell)
	if err != nil {
		s.Fatal("Failed to find calendar first Monday cell bounds: ", err)
	}
	cellPositionY = 0
	for i := 0; i < findCellTimes; i++ {
		s.Logf("Moving towards to the first Tuesday cell (iteration %d of %d)", i+1, findCellTimes)
		cellPositionY += 5
		firstTuesdayDateCellPt := coords.NewPoint(firstTuesdayDateCellBounds.CenterX(), scrollViewBounds.Top+cellPositionY)
		if err := mouse.Click(tconn, firstTuesdayDateCellPt, mouse.LeftButton)(ctx); err != nil {
			s.Fatal("Failed to click the first Tuesday date cell: ", err)
		}
		if found, err := ui.IsNodeFound(ctx, eventCloseButtonView); err != nil {
			s.Fatal("Failed to check event list view close button while finding the first Tuesday cell: ", err)
		} else if found == true {
			break
		}
	}

	// Calendar list view should show "Open in Google calendar" if there's no events.
	openButtonContentView := nodewith.HasClass("View").Ancestor(eventContentView).Nth(0)
	openButtonView := nodewith.HasClass("PillButton").Ancestor(openButtonContentView)
	openButtonLabelView := nodewith.Name("Open in Google Calendar").HasClass("LabelButtonLabel").Ancestor(openButtonView)
	if err := ui.WaitUntilExists(openButtonLabelView)(ctx); err != nil {
		s.Fatal("Failed to find the Open in Google calendar label after clicking on the first Tuesday date cell: ", err)
	}

	// Close event list view.
	if err := ui.LeftClick(eventCloseButtonView)(ctx); err != nil {
		s.Fatal("Failed to click the close button in calendar event list view: ", err)
	}

	// Close the calendar view.
	calendarViewBounds, err := ui.Location(ctx, calendarView)
	if err != nil {
		s.Fatal("Failed to find calendar view bounds: ", err)
	}
	outsideCalendarPt := coords.NewPoint(calendarViewBounds.Right()+2, calendarViewBounds.Top-5)
	if err := mouse.Click(tconn, outsideCalendarPt, mouse.LeftButton)(ctx); err != nil {
		s.Fatal("Failed to click outside of the calendar view: ", err)
	}
}
