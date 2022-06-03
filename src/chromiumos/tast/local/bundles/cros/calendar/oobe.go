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
	"chromiumos/tast/local/chrome/uiauto/faillog"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         Oobe,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the basic interacting with calendar view",
		Contacts: []string{
			"jiamingc@chromium.org",
			"cros-calendar@google.com",
		},
		VarDeps: []string{
			"ui.signinProfileTestExtensionManifestKey",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
	})
}

// Oobe test verifies that we can open the calendar with shoulw the current year label correctly.
// But it should not show the event list when clicking on a deat cell.
func Oobe(ctx context.Context, s *testing.State) {
	cr, err := chrome.New(
		ctx,
		chrome.Region("us"),
		chrome.LoadSigninProfileExtension(s.RequiredVar("ui.signinProfileTestExtensionManifestKey")),
		chrome.EnableFeatures("CalendarView"),
		chrome.NoLogin())
	if err != nil {
		s.Fatal("Failed to start Chrome: ", err)
	}
	defer cr.Close(ctx)

	oobeConn, err := cr.WaitForOOBEConnection(ctx)
	if err != nil {
		s.Fatal("Failed to create OOBE connection: ", err)
	}
	defer oobeConn.Close()

	tconn, err := cr.SigninProfileTestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create the signin profile test API connection: ", err)
	}
	defer faillog.DumpUITreeOnError(ctx, s.OutDir(), s.HasError, tconn)

	s.Log("Waiting for the welcome screen")
	if err := oobeConn.WaitForExprFailOnErr(ctx, "OobeAPI.screens.WelcomeScreen.isVisible()"); err != nil {
		s.Fatal("Failed to wait for the welcome screen to be visible: ", err)
	}

	ui := uiauto.New(tconn)

	s.Log("Start testing calendar view from date tray")
	dateTray := nodewith.ClassName("DateTray")

	// Comparing the time before and after opening the calendar view just in case this test is run at the very end of a year, e.g. Dec 31 23:59:59.
	beforeOpeningCalendarYear := time.Now().Year()

	if err := ui.WaitUntilExists(dateTray.Onscreen())(ctx); err != nil {
		s.Fatal("Failed to find date tray: ", err)
	}

	// Open calendar view.
	if err := ui.DoDefault(dateTray)(ctx); err != nil {
		s.Fatal("Failed to click the date tray: ", err)
	}

	calendarView := nodewith.ClassName("CalendarView")
	mainHeaderTriView := nodewith.ClassName("TriView").Ancestor(calendarView).Nth(0)
	mainHeaderContainer := nodewith.ClassName("View").Ancestor(mainHeaderTriView).Nth(1)
	mainHeader := nodewith.Name("Calendar").ClassName("Label").Ancestor(mainHeaderContainer)

	if err := ui.WaitUntilExists(mainHeader)(ctx); err != nil {
		s.Fatal("Failed to find calendar main label after opening calendar view: ", err)
	}

	// For some corner cases, if it cannot find the year label with the time before clicking on the date tray, it should find the year label with the time after the calendar view is open.
	// E.g. before opening it's Dec 31 23:59:59 2022, and after openting it's Jan 1 00:00 2023.
	yearInt := beforeOpeningCalendarYear
	beforeOpeningCalendarYearLabel := nodewith.Name(strconv.Itoa(beforeOpeningCalendarYear)).ClassName("Label").Onscreen()
	if found, err := ui.IsNodeFound(ctx, beforeOpeningCalendarYearLabel); err != nil {
		s.Fatal("Failed to check beforeOpeningCalendarYearLabel after clicking on the date tray: ", err)
	} else if found != true {
		yearInt = time.Now().Year()
	}

	// Opening the calendar view should show today's year label.
	year := strconv.Itoa(yearInt)
	todayYearLabel := nodewith.Name(year).ClassName("Label").Onscreen()
	if err := ui.WaitUntilExists(todayYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after opening calendar view: ", err)
	}

	// Clicking on a Monday's cell should not show the event list view.
	scrollView := nodewith.ClassName("ScrollView").Ancestor(calendarView).Nth(0)
	scrollViewport := nodewith.ClassName("ScrollView::Viewport").Ancestor(scrollView).Nth(0)
	contentView := nodewith.ClassName("View").Ancestor(scrollViewport).Nth(0)
	currentMonthView := nodewith.ClassName("View").Ancestor(contentView).Nth(3)
	firstMondayDateCell := nodewith.ClassName("CalendarDateCellView").Ancestor(currentMonthView).Nth(1)
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
	eventListView := nodewith.ClassName("CalendarEventListView").Ancestor(calendarView)
	eventCloseButtonContentView := nodewith.ClassName("View").Ancestor(eventListView).Nth(0)
	for i := 0; i < findCellTimes; i++ {
		s.Logf("Moving towards to the first Monday cell (iteration %d of %d)", i+1, findCellTimes)
		cellPositionY += 5
		firstMondayDateCellPt := coords.NewPoint(firstMondayDateCellBounds.CenterX(), scrollViewBounds.Top+cellPositionY)
		if err := mouse.Click(tconn, firstMondayDateCellPt, mouse.LeftButton)(ctx); err != nil {
			s.Fatal("Failed to click the first Monay date cell: ", err)
		}
		if found, err := ui.IsNodeFound(ctx, eventCloseButtonContentView); err != nil {
			s.Fatal("Failed to check event list view close button while finding the first Monday cell: ", err)
		} else if found == true {
			// If it shows the event list view, it's a bug.
			s.Fatal("Should not open event list after clicking the first Monay date cell: ", err)
			break
		}
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
