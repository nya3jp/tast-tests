// Copyright 2022 The ChromiumOS Authors.
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
	dateTray := nodewith.ClassName("DateTray")

	// Opening the calendar view should show today's year label.
	if err := ui.LeftClick(dateTray)(ctx); err != nil {
		s.Fatal("Failed to click the date tray: ", err)
	}

	year := strconv.Itoa(time.Now().Year())
	todayYearLabel := nodewith.Name(year).ClassName("Label").Onscreen()
	if err := ui.WaitUntilExists(todayYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after opening calendar view: ", err)
	}

	if err := uiauto.Sleep(3 * time.Second)(ctx); err != nil {
		s.Fatal("Failed to wait for one second: ", err)
	}

	// Click on a Monday's cell to show the event list view.
	calendarView := nodewith.ClassName("CalendarView")
	scrollView := nodewith.ClassName("ScrollView").Ancestor(calendarView).Nth(0)
	scrollViewport := nodewith.ClassName("ScrollView::Viewport").Ancestor(scrollView).Nth(0)
	contentView := nodewith.ClassName("View").Ancestor(scrollViewport).Nth(0)
	currentMonthView := nodewith.ClassName("View").Ancestor(contentView).Nth(3)
	firstMondayDateCell := nodewith.ClassName("CalendarDateCellView").Ancestor(currentMonthView).Nth(1)
	scrollViewBounds, err := ui.Location(ctx, scrollView)
	if err != nil {
		s.Fatal("Failed to find calendar scroll view bounds: ", err)
	}

	if err := ui.MakeVisible(firstMondayDateCell)(ctx); err != nil {
		s.Fatal("Failed to make first monday date cell visible: ", err)
	}

	firstMondayDateCellBounds, err := ui.Location(ctx, firstMondayDateCell)
	if err != nil {
		s.Fatal("Failed to find calendar first Monday cell bounds: ", err)
	}

	s.Log("firstMondayDateCellBounds is ", firstMondayDateCellBounds)
	s.Log("scroll view bounds is ", scrollViewBounds)

	// The vertical location of the cells fetched from |ui.Location| are not correct.
	// It returns the same number for the |Top| of all the cells, which is the same number as the |Top| of the scroll view.
	// This might because of the scroll view is nested in some other views.
	// Here a small amount (5) of pixel is added to the top of the scroll view each time in the loop to find the first available Monday cell with events.
	const findMondayCellTimes = 10
	cellPositionY := 0
	eventListView := nodewith.ClassName("CalendarEventListView").Ancestor(calendarView)
	eventCloseButtonContentView := nodewith.ClassName("View").Ancestor(eventListView).Nth(0)
	eventCloseButtonView := nodewith.ClassName("ImageButton").Ancestor(eventCloseButtonContentView).Nth(0)
	for i := 0; i < findMondayCellTimes; i++ {
		s.Logf("Moving towards to the first Monday cell (iteration %d of %d)", i+1, findMondayCellTimes)
		cellPositionY += 5
		firstMondayDateCellPt := coords.NewPoint(firstMondayDateCellBounds.CenterX(), scrollViewBounds.Top+cellPositionY)
		if err := mouse.Click(tconn, firstMondayDateCellPt, mouse.LeftButton)(ctx); err != nil {
			s.Fatal("Failed to click the first Monay date cell: ", err)
		}
		if found, err := ui.IsNodeFound(ctx, eventCloseButtonView); err != nil {
			s.Fatal("Failed to check event list view close button while finding the first Monday cell: ", err)
		} else if found == true {
			break
		}
	}
}
