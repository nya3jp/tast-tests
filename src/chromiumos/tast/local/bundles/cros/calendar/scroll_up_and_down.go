// Copyright 2022 The ChromiumOS Authors
// Use of this source code is governed by a BSD-style license that can be
// found in the LICENSE file.

package calendar

import (
	"context"
	"strconv"
	"time"

	"chromiumos/tast/local/chrome"
	"chromiumos/tast/local/chrome/ash"
	"chromiumos/tast/local/chrome/uiauto"
	"chromiumos/tast/local/chrome/uiauto/mouse"
	"chromiumos/tast/local/chrome/uiauto/nodewith"
	"chromiumos/tast/local/chrome/uiauto/touch"
	"chromiumos/tast/local/coords"
	"chromiumos/tast/testing"
	"chromiumos/tast/testing/hwdep"
)

func init() {
	testing.AddTest(&testing.Test{
		Func:         ScrollUpAndDown,
		LacrosStatus: testing.LacrosVariantExists,
		Desc:         "Checks the scroll to next/previous months on the calendar view",
		Contacts: []string{
			"jiamingc@chromium.org",
			"cros-calendar@google.com",
		},
		Attr:         []string{"group:mainline", "informational"},
		SoftwareDeps: []string{"chrome"},
		Fixture:      "chromeLoggedInWithCalendarView",
		HardwareDeps: hwdep.D(hwdep.InternalDisplay()),
	})
}

// ScrollUpAndDown verifies that we can open the calendar, and scroll up/down then back to today correctly.
func ScrollUpAndDown(ctx context.Context, s *testing.State) {
	cr := s.FixtValue().(*chrome.Chrome)

	tconn, err := cr.TestAPIConn(ctx)
	if err != nil {
		s.Fatal("Failed to create Test API connection: ", err)
	}

	cleanup, err := ash.EnsureTabletModeEnabled(ctx, tconn, true)
	if err != nil {
		s.Fatal("Failed to ensure in tablet mode: ", err)
	}
	defer cleanup(ctx)

	tc, err := touch.New(ctx, tconn)
	if err != nil {
		s.Fatal("Failed to set up the touch context: ", err)
	}
	defer tc.Close()

	s.Log("Start testing calendar view from date tray")
	ui := uiauto.New(tconn)

	dateTray := nodewith.ClassName("DateTray")

	// Comparing the time before and after opening the calendar view just in case this test is run at the very end of a year, e.g. Dec 31 23:59:59.
	beforeOpeningCalendarYear := time.Now().Year()

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

	scrollView := nodewith.ClassName("ScrollView").Ancestor(calendarView).Nth(0)
	scrollViewBounds, err := ui.Location(ctx, scrollView)
	if err != nil {
		s.Fatal("Failed to find calendar scroll view bounds: ", err)
	}

	// Sets swipe points and speed.
	swipeStartPt := coords.NewPoint(scrollViewBounds.CenterX(), scrollViewBounds.Top+20)
	swipeEndPt := coords.NewPoint(scrollViewBounds.CenterX(), scrollViewBounds.Bottom()-10)
	const swipeSpeed = 10 * time.Millisecond
	const delay = 1 * time.Second

	// The scroll times depends on the screen size.
	// But it should scroll one month with at most 3 scrolls.
	// So here 3*12 (months) is set as the max scroll times.
	const scrollTimes = 36

	// Scroll until previous year label is visible.
	previousYear := strconv.Itoa(yearInt - 1)
	previousYearLabel := nodewith.Name(previousYear).ClassName("Label").Onscreen()
	for i := 0; i < scrollTimes; i++ {
		if err := tc.Swipe(swipeStartPt, tc.SwipeTo(swipeEndPt, swipeSpeed), tc.Hold(delay))(ctx); err != nil {
			s.Fatal("Failed to scroll up on calendar view: ", err)
		}
		if found, err := ui.IsNodeFound(ctx, previousYearLabel); err != nil {
			s.Fatal("Failed to check previous year while scrolling: ", err)
		} else if found == true {
			break
		}
	}

	if err := ui.WaitUntilExists(previousYearLabel)(ctx); err != nil {
		s.Fatal("Failed to scroll previous year: ", err)
	}

	// Clicking on today button should go back to today's momth.
	todayButton := nodewith.NameContaining("Today").ClassName("PillButton")
	beforeClickingTodayYear := time.Now().Year()
	if err := ui.LeftClick(todayButton)(ctx); err != nil {
		s.Fatal("Failed to click the today button in calendar view bubble: ", err)
	}
	if err := ui.WaitUntilGone(previousYearLabel)(ctx); err != nil {
		s.Fatal("Failed to wait for the year label to be stable after click on today button: ", err)
	}
	yearInt = beforeClickingTodayYear
	year = strconv.Itoa(yearInt)
	todayYearLabel = nodewith.Name(year).ClassName("Label").Onscreen()
	if found, err := ui.IsNodeFound(ctx, todayYearLabel); err != nil {
		s.Fatal("Failed to check today year label with beforeClickingTodayYear after clicking on the today button: ", err)
	} else if found != true {
		yearInt = time.Now().Year()
		year = strconv.Itoa(yearInt)
		todayYearLabel = nodewith.Name(year).ClassName("Label").Onscreen()
	}
	if err := ui.WaitUntilExists(todayYearLabel)(ctx); err != nil {
		s.Fatal("Failed to find year label after clicking on today button: ", err)
	}

	// Scroll until next year label is visible.
	nextYear := strconv.Itoa(yearInt + 1)
	nextYearLabel := nodewith.Name(nextYear).ClassName("Label").Onscreen()
	for i := 0; i < scrollTimes; i++ {
		if err := tc.Swipe(swipeEndPt, tc.SwipeTo(swipeStartPt, swipeSpeed), tc.Hold(delay))(ctx); err != nil {
			s.Fatal("Failed to scroll down on calendar view: ", err)
		}
		if found, err := ui.IsNodeFound(ctx, nextYearLabel); err != nil {
			s.Fatal("Failed to check next year while scrolling: ", err)
		} else if found == true {
			break
		}
	}
	if err := ui.WaitUntilExists(nextYearLabel)(ctx); err != nil {
		s.Fatal("Failed to scroll next year: ", err)
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
